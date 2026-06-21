"""Unit tests for the combined Go and Python coverage gate."""

from __future__ import annotations

from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch
import unittest

from tools import coverage_gate


class ModulePathTest(unittest.TestCase):
    def test_resolves_module_name_inside_repo(self) -> None:
        repo_root = Path("/tmp/repo")
        self.assertEqual(
            coverage_gate.module_path(repo_root, "tools.coverage_gate"),
            repo_root / "tools" / "coverage_gate.py",
        )


class ExecutableLinesTest(unittest.TestCase):
    def test_collects_executable_lines_from_function_bodies(self) -> None:
        with TemporaryDirectory() as tmp:
            path = Path(tmp) / "sample.py"
            path.write_text(
                "\n".join(
                    [
                        '"""module doc"""',
                        "",
                        "def sample():",
                        '    """docstring"""',
                        "    value = 1",
                        "    if value:",
                        "        return value",
                    ]
                ),
                encoding="utf-8",
            )

            lines = coverage_gate.executable_lines(path)

        self.assertIn(5, lines)
        self.assertIn(6, lines)
        self.assertIn(7, lines)


class ParseGoCoverOutputTest(unittest.TestCase):
    def test_counts_covered_and_total_statements(self) -> None:
        output = "\n".join(
            [
                "mode: set",
                "internal/schema/parse.go:10.1,12.2 2 1",
                "internal/schema/parse.go:14.1,16.2 3 0",
                "internal/registry/resolve.go:20.1,21.2 5 7",
            ]
        )

        covered, total = coverage_gate.parse_go_cover_output(output)

        self.assertEqual((covered, total), (7, 10))

    def test_rejects_unexpected_line_shape(self) -> None:
        with self.assertRaisesRegex(ValueError, "unexpected coverprofile line"):
            coverage_gate.parse_go_cover_output("mode: set\nnot-a-cover-line\n")


class PythonCoverageSummaryTest(unittest.TestCase):
    def test_returns_hit_counts_and_rows(self) -> None:
        repo_root = Path("/repo")

        class FakeTracer:
            def results(self) -> object:
                return type(
                    "Results",
                    (),
                    {"counts": {("/repo/tools/a.py", 10): 1, ("/repo/tools/b.py", 20): 1}},
                )()

        with patch.object(coverage_gate, "PYTHON_TARGET_MODULES", ["tools.a", "tools.b"]):
            with patch.object(coverage_gate, "module_path") as module_path:
                with patch.object(coverage_gate, "executable_lines") as executable_lines:
                    module_path.side_effect = [Path("/repo/tools/a.py"), Path("/repo/tools/b.py")]
                    executable_lines.side_effect = [{10, 11}, {20}]

                    hit, total, rows = coverage_gate.python_coverage_summary(
                        repo_root, FakeTracer()
                    )

        self.assertEqual((hit, total), (2, 3))
        self.assertEqual(len(rows), 2)
        self.assertIn("tools.a: 1/2 lines (50.0%)", rows)
        self.assertIn("tools.b: 1/1 lines (100.0%)", rows)


class RunGoTestsWithCoverageTest(unittest.TestCase):
    def test_runs_go_tests_and_summarizes_profile(self) -> None:
        repo_root = Path("/repo")

        def fake_run(cmd: list[str], cwd: Path, check: bool) -> None:
            self.assertEqual(cmd[0], "go")
            self.assertEqual(cwd, repo_root)
            self.assertTrue(check)
            profile_arg = next(arg for arg in cmd if arg.startswith("-coverprofile="))
            profile_path = Path(profile_arg.split("=", 1)[1])
            profile_path.write_text(
                "mode: set\ninternal/config/settings.go:1.1,2.2 3 1\n",
                encoding="utf-8",
            )

        with patch.object(coverage_gate.subprocess, "run", side_effect=fake_run):
            covered, total, row = coverage_gate.run_go_tests_with_coverage(repo_root)

        self.assertEqual((covered, total), (3, 3))
        self.assertEqual(row, "go total: 3/3 statements (100.0%)")


class MainTest(unittest.TestCase):
    def test_returns_one_when_python_suite_fails(self) -> None:
        fake_result = type("Result", (), {"wasSuccessful": lambda self: False})()

        with patch.object(coverage_gate, "run_go_tests_with_coverage", return_value=(1, 1, "go")):
            with patch.object(
                coverage_gate, "run_suite_with_trace", return_value=(fake_result, object())
            ):
                exit_code = coverage_gate.main(["90"])

        self.assertEqual(exit_code, 1)

    def test_returns_one_when_combined_coverage_is_below_minimum(self) -> None:
        fake_result = type("Result", (), {"wasSuccessful": lambda self: True})()

        with patch.object(
            coverage_gate, "run_go_tests_with_coverage", return_value=(5, 10, "go total: 5/10 statements (50.0%)")
        ):
            with patch.object(
                coverage_gate, "run_suite_with_trace", return_value=(fake_result, object())
            ):
                with patch.object(
                    coverage_gate,
                    "python_coverage_summary",
                    return_value=(5, 10, ["tools.coverage_gate: 5/10 lines (50.0%)"]),
                ):
                    with patch("sys.stdout") as mock_stdout:
                        exit_code = coverage_gate.main(["90"])

        self.assertEqual(exit_code, 1)

    def test_returns_zero_when_combined_coverage_meets_minimum(self) -> None:
        fake_result = type("Result", (), {"wasSuccessful": lambda self: True})()

        with patch.object(
            coverage_gate, "run_go_tests_with_coverage", return_value=(9, 10, "go total: 9/10 statements (90.0%)")
        ):
            with patch.object(
                coverage_gate, "run_suite_with_trace", return_value=(fake_result, object())
            ):
                with patch.object(
                    coverage_gate,
                    "python_coverage_summary",
                    return_value=(9, 10, ["tools.coverage_gate: 9/10 lines (90.0%)"]),
                ):
                    with patch("sys.stdout") as mock_stdout:
                        exit_code = coverage_gate.main(["90"])

        self.assertEqual(exit_code, 0)


if __name__ == "__main__":
    unittest.main()
