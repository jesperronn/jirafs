"""Unit tests for repository check helpers."""

from __future__ import annotations

from pathlib import Path
from tempfile import TemporaryDirectory
import unittest

from tools import repo_checks


class MarkdownRelativeLinksTest(unittest.TestCase):
    def test_filters_urls_and_anchors(self) -> None:
        text = (
            "[Architecture](docs/architecture.md)\n"
            "[External](https://example.com)\n"
            "[Anchor](#goal)\n"
        )

        self.assertEqual(
            repo_checks.markdown_relative_links(text),
            ["docs/architecture.md"],
        )


class RepoRootFromTest(unittest.TestCase):
    def test_resolves_repo_root_for_package_init(self) -> None:
        path = Path("/tmp/example/tools/__init__.py")
        self.assertEqual(repo_checks.repo_root_from(path), path.resolve().parent.parent)

    def test_returns_resolved_path_for_other_files(self) -> None:
        path = Path("/tmp/example/README.md")
        self.assertEqual(repo_checks.repo_root_from(path), path.resolve())


class RequiredFilesMissingTest(unittest.TestCase):
    def test_reports_missing_required_files(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            missing = repo_checks.required_files_missing(repo_root)

        self.assertIn("README.md", missing)
        self.assertIn(".github/workflows/ci.yml", missing)

    def test_returns_empty_when_required_files_exist(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            for relative_path in repo_checks.DOC_FILES + repo_checks.SCRIPT_FILES:
                path = repo_root / relative_path
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("x\n", encoding="utf-8")
            workflow = repo_root / ".github/workflows/ci.yml"
            workflow.parent.mkdir(parents=True, exist_ok=True)
            workflow.write_text("steps:\n", encoding="utf-8")

            missing = repo_checks.required_files_missing(repo_root)

        self.assertEqual(missing, [])


class BrokenMarkdownLinksTest(unittest.TestCase):
    def test_reports_missing_relative_target(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            docs_dir = repo_root / "docs"
            docs_dir.mkdir(parents=True)
            (docs_dir / "a.md").write_text("[B](b.md)\n", encoding="utf-8")

            broken = repo_checks.broken_markdown_links(repo_root, "docs/a.md")

        self.assertEqual(broken, ["b.md"])

    def test_accepts_existing_relative_target(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            docs_dir = repo_root / "docs"
            docs_dir.mkdir(parents=True)
            (docs_dir / "a.md").write_text("[B](b.md)\n", encoding="utf-8")
            (docs_dir / "b.md").write_text("# B\n", encoding="utf-8")

            broken = repo_checks.broken_markdown_links(repo_root, "docs/a.md")

        self.assertEqual(broken, [])


class TextHelpersTest(unittest.TestCase):
    def test_trailing_whitespace_detection(self) -> None:
        self.assertTrue(repo_checks.has_trailing_whitespace("hello  \nworld\n"))
        self.assertFalse(repo_checks.has_trailing_whitespace("hello\nworld\n"))

    def test_tab_detection(self) -> None:
        self.assertTrue(repo_checks.file_has_tabs("a\tb"))
        self.assertFalse(repo_checks.file_has_tabs("ab"))

    def test_shebang_detection(self) -> None:
        self.assertTrue(repo_checks.shebang_is_present("#!/usr/bin/env bash\n"))
        self.assertFalse(repo_checks.shebang_is_present("echo test\n"))


class CiWrapperCheckTest(unittest.TestCase):
    def test_ci_requires_local_wrapper_commands(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            workflow = repo_root / ".github" / "workflows"
            workflow.mkdir(parents=True)
            (workflow / "ci.yml").write_text(
                "steps:\n"
                "  - run: bin/lint\n"
                "  - run: bin/test\n",
                encoding="utf-8",
            )

            self.assertTrue(repo_checks.ci_uses_local_wrappers(repo_root))

    def test_ci_wrapper_check_fails_without_local_commands(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            workflow = repo_root / ".github" / "workflows"
            workflow.mkdir(parents=True)
            (workflow / "ci.yml").write_text(
                "steps:\n"
                "  - run: python -m unittest\n",
                encoding="utf-8",
            )

            self.assertFalse(repo_checks.ci_uses_local_wrappers(repo_root))


class ReadHelpersTest(unittest.TestCase):
    def test_read_text_returns_file_content(self) -> None:
        with TemporaryDirectory() as tmp:
            path = Path(tmp) / "note.txt"
            path.write_text("hello\n", encoding="utf-8")

            self.assertEqual(repo_checks.read_text(path), "hello\n")

    def test_readme_document_links_returns_relative_targets(self) -> None:
        with TemporaryDirectory() as tmp:
            repo_root = Path(tmp)
            (repo_root / "README.md").write_text(
                "[Architecture](docs/architecture.md)\n"
                "[CLI](docs/cli.md)\n",
                encoding="utf-8",
            )

            self.assertEqual(
                repo_checks.readme_document_links(repo_root),
                ["docs/architecture.md", "docs/cli.md"],
            )


if __name__ == "__main__":
    unittest.main()
