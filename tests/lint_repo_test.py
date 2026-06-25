"""Unit tests for repository lint orchestration."""

from __future__ import annotations

from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch
import os
import unittest

from tools import lint_repo


MINIMAL_DOCS = {
    "README.md": "[Architecture](docs/architecture.md)\n",
    "docs/architecture.md": "# A\n",
    "docs/issue-format.md": "# B\n",
    "docs/sync-model.md": "# C\n",
    "docs/references.md": "# D\n",
    "docs/templates.md": "# E\n",
    "docs/cli.md": "# F\n",
    "docs/agent-skill-integration.md": "# G\n",
    "docs/implementation-roadmap.md": "# H\n",
    "docs/parallel-workstreams.md": "# I\n",
    "docs/orchestration-model.md": "# J\n",
    "docs/verification-policy.md": "# K\n",
    "docs/development-rules.md": "# L\n",
    "docs/code-style.md": "# M\n",
    "docs/validator-contract.md": "# N\n",
    "docs/mirror-model.md": "# O\n",
    "docs/settings-and-context.md": "# P\n",
    "docs/credential-sources.md": "# Q\n",
    "docs/project-selection-cli.md": "# R\n",
    "docs/implementation-packets.md": "# S\n",
    "docs/ralph-task-archive.md": "# T\n",
    "docs/ralph-stream-ws1-schema-codec.md": "# U\n",
    "docs/ralph-stream-ws2-settings-references.md": "# V\n",
    "docs/ralph-stream-ws3-jira-export.md": "# W\n",
    "docs/ralph-parallel-workflow.md": "# X\n",
}

MINIMAL_SCRIPTS = {
    "bin/integrate_stream_commit": "#!/usr/bin/env bash\n",
    "bin/lint": "#!/usr/bin/env bash\n",
    "bin/test": "#!/usr/bin/env bash\n",
    "tools/repo_checks.py": "x = 1\n",
    "tools/lint_repo.py": "x = 1\n",
    "tools/coverage_gate.py": "x = 1\n",
    ".github/workflows/ci.yml": "steps:\n  - run: bin/lint\n  - run: bin/test\n",
}


def build_repo(repo_root: Path) -> None:
    """Create a minimal repository layout for lint testing."""
    for relative_path, content in {**MINIMAL_DOCS, **MINIMAL_SCRIPTS}.items():
        path = repo_root / relative_path
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        if relative_path in {"bin/integrate_stream_commit", "bin/lint", "bin/test"}:
            os.chmod(path, 0o755)


class LintRepoRunTest(unittest.TestCase):
    def test_clean_repository_has_no_errors(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)

            errors = lint_repo.run(repo_root)

        self.assertEqual(errors, [])

    def test_missing_ci_wrapper_is_reported(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / ".github/workflows/ci.yml").write_text(
                "steps:\n  - run: python -m unittest\n",
                encoding="utf-8",
            )

            errors = lint_repo.check_ci(repo_root)

        self.assertEqual(
            errors,
            [".github/workflows/ci.yml: must run bin/lint and bin/test"],
        )

    def test_missing_required_files_are_reported(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)

            errors = lint_repo.check_required_files(repo_root)

        self.assertIn("missing required file: README.md", errors)

    def test_broken_markdown_links_are_reported(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / "docs/cli.md").write_text("[Broken](missing.md)\n", encoding="utf-8")

            errors = lint_repo.check_markdown_links(repo_root)

        self.assertEqual(errors, ["docs/cli.md: broken link target missing.md"])

    def test_text_hygiene_detects_tabs_and_trailing_spaces(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / "docs/cli.md").write_text("hello \n\tworld\n", encoding="utf-8")

            errors = lint_repo.check_text_hygiene(repo_root)

        self.assertIn("docs/cli.md: trailing whitespace", errors)
        self.assertIn("docs/cli.md: hard tabs are not allowed", errors)

    def test_non_executable_script_is_reported(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            os.chmod(repo_root / "bin/test", 0o644)

            errors = lint_repo.check_scripts(repo_root)

        self.assertIn("bin/test: script is not executable", errors)

    def test_missing_shebang_is_reported(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / "bin/lint").write_text("echo lint\n", encoding="utf-8")
            os.chmod(repo_root / "bin/lint", 0o755)

            errors = lint_repo.check_scripts(repo_root)

        self.assertIn("bin/lint: missing shebang", errors)

    def test_run_aggregates_all_check_errors(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)

            errors = lint_repo.run(repo_root)

        self.assertTrue(errors)
        self.assertTrue(any(error.startswith("missing required file:") for error in errors))

    def test_main_returns_zero_when_clean(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)

            with patch.object(lint_repo, "__file__", str(repo_root / "tools/lint_repo.py")):
                self.assertEqual(lint_repo.main(), 0)

    def test_main_returns_one_when_dirty(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / ".github/workflows/ci.yml").write_text("steps:\n", encoding="utf-8")

            with patch.object(lint_repo, "__file__", str(repo_root / "tools/lint_repo.py")):
                self.assertEqual(lint_repo.main(), 1)

    def test_main_prints_errors_to_stderr(self) -> None:
        """Verify that main() prints lint errors to stderr when repo is dirty."""
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            build_repo(repo_root)
            (repo_root / ".github/workflows/ci.yml").write_text(
                "steps:\n  - run: python -m unittest\n",
                encoding="utf-8",
            )

            # Patch sys.stderr to capture output.
            import io

            captured = io.StringIO()
            with patch.object(lint_repo, "__file__", str(repo_root / "tools/lint_repo.py")):
                with patch("sys.stderr", captured):
                    lint_repo.main()

            stderr_text = captured.getvalue()
            self.assertIn("must run bin/lint and bin/test", stderr_text)


if __name__ == "__main__":
    unittest.main()
