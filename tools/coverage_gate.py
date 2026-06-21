"""Run Go and Python tests and enforce a combined coverage floor."""

from __future__ import annotations

from importlib import util
from pathlib import Path
import ast
import os
import re
import subprocess
import sys
import tempfile
import trace
import unittest


PYTHON_TARGET_MODULES = [
    "tools.coverage_gate",
    "tools.repo_checks",
    "tools.lint_repo",
]
GO_COVER_TOTAL_RE = re.compile(r"^total:\s+\(statements\)\s+(\d+(?:\.\d+)?)%$")


def module_path(repo_root: Path, module_name: str) -> Path:
    """Resolve a module name to a file path inside the repo."""
    return repo_root / Path(*module_name.split(".")).with_suffix(".py")


def executable_lines(path: Path) -> set[int]:
    """Return executable line numbers for a Python source file."""
    source = path.read_text(encoding="utf-8")
    tree = ast.parse(source, filename=str(path))
    lines: set[int] = set()
    for node in ast.walk(tree):
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            continue
        for child in ast.walk(node):
            if isinstance(child, ast.stmt) and not isinstance(
                child,
                (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef, ast.Pass),
            ):
                if isinstance(child, ast.Expr) and isinstance(
                    getattr(child, "value", None),
                    ast.Constant,
                ) and isinstance(child.value.value, str):
                    continue
                lines.add(child.lineno)
    return lines


def load_suite(repo_root: Path) -> unittest.TestSuite:
    """Discover the repository test suite."""
    loader = unittest.defaultTestLoader
    return loader.discover(str(repo_root / "tests"), pattern="*_test.py")


def run_suite_with_trace(repo_root: Path) -> tuple[unittest.result.TestResult, trace.Trace]:
    """Run the suite under tracing and return the result plus tracer."""
    suite = load_suite(repo_root)
    tracer = trace.Trace(count=True, trace=False)

    def _run() -> unittest.result.TestResult:
        runner = unittest.TextTestRunner(verbosity=2)
        return runner.run(suite)

    result = tracer.runfunc(_run)
    if not isinstance(result, unittest.result.TestResult):
        raise TypeError("expected unittest result from traced run")
    return result, tracer


def python_coverage_summary(
    repo_root: Path,
    tracer: trace.Trace,
) -> tuple[int, int, list[str]]:
    """Return Python hit/executable counts plus per-module rows."""
    results = tracer.results()
    counts = results.counts
    rows: list[str] = []
    total_executable = 0
    total_hit = 0

    for module_name in PYTHON_TARGET_MODULES:
        path = module_path(repo_root, module_name)
        executable = executable_lines(path)
        hit = sum(1 for line in executable if counts.get((str(path), line), 0) > 0)
        total_executable += len(executable)
        total_hit += hit
        percent = 100.0 if not executable else (hit / len(executable)) * 100.0
        rows.append(
            f"{module_name}: {hit}/{len(executable)} lines ({percent:.1f}%)"
        )

    return total_hit, total_executable, rows


def parse_go_cover_output(output: str) -> tuple[int, int]:
    """Return covered and total Go statement counts from a cover profile."""
    covered = 0
    total = 0

    for line in output.splitlines():
        if line.startswith("mode:"):
            continue
        parts = line.split()
        if len(parts) != 3:
            raise ValueError(f"unexpected coverprofile line: {line!r}")
        num_statements = int(parts[1])
        count = int(parts[2])
        total += num_statements
        if count > 0:
            covered += num_statements

    return covered, total


def run_go_tests_with_coverage(repo_root: Path) -> tuple[int, int, str]:
    """Run Go tests with a cover profile and return covered/total statements."""
    with tempfile.NamedTemporaryFile(
        prefix="jirafs-go-cover-", suffix=".out", delete=False
    ) as tmp:
        coverprofile = Path(tmp.name)

    try:
        subprocess.run(
            ["go", "test", f"-coverprofile={coverprofile}", "./..."],
            cwd=repo_root,
            check=True,
        )
        profile_text = coverprofile.read_text(encoding="utf-8")
        covered, total = parse_go_cover_output(profile_text)
    finally:
        try:
            os.unlink(coverprofile)
        except FileNotFoundError:
            pass

    percent = 100.0 if total == 0 else (covered / total) * 100.0
    row = f"go total: {covered}/{total} statements ({percent:.1f}%)"
    return covered, total, row


def main(argv: list[str] | None = None) -> int:
    """Run the suites and fail if combined coverage drops below the threshold."""
    args = argv or sys.argv[1:]
    minimum = float(args[0]) if args else 90.0
    repo_root = Path(__file__).resolve().parent.parent

    go_hit, go_total, go_row = run_go_tests_with_coverage(repo_root)
    result, tracer = run_suite_with_trace(repo_root)
    if not result.wasSuccessful():
        return 1

    py_hit, py_total, rows = python_coverage_summary(repo_root, tracer)
    total_hit = go_hit + py_hit
    total_executable = go_total + py_total
    total = 100.0 if total_executable == 0 else (total_hit / total_executable) * 100.0

    if total < minimum:
        print(go_row)
        for row in rows:
            print(row)
        python_percent = 100.0 if py_total == 0 else (py_hit / py_total) * 100.0
        print(f"python total: {py_hit}/{py_total} lines ({python_percent:.1f}%)")
        print(f"total: {total:.1f}%")
        print(
            f"coverage gate failed: total {total:.1f}% is below required {minimum:.1f}%",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
