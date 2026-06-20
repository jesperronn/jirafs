"""Shared repository checks used by lint, tests, and CI."""

from __future__ import annotations

from pathlib import Path
import re


DOC_FILES = [
    "README.md",
    "docs/architecture.md",
    "docs/issue-format.md",
    "docs/sync-model.md",
    "docs/references.md",
    "docs/templates.md",
    "docs/cli.md",
    "docs/agent-skill-integration.md",
    "docs/implementation-roadmap.md",
    "docs/parallel-workstreams.md",
    "docs/verification-policy.md",
    "docs/development-rules.md",
    "docs/mirror-model.md",
    "docs/settings-and-context.md",
    "docs/credential-sources.md",
    "docs/project-selection-cli.md",
    "docs/implementation-packets.md",
]

SCRIPT_FILES = [
    "bin/lint",
    "bin/test",
    "tools/repo_checks.py",
    "tools/lint_repo.py",
    "tools/coverage_gate.py",
]

MARKDOWN_LINK_RE = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")


def repo_root_from(path: Path) -> Path:
    """Return the repository root for a file inside this repo."""
    return path.resolve().parent.parent if path.name == "__init__.py" else path.resolve()


def read_text(path: Path) -> str:
    """Read a UTF-8 text file."""
    return path.read_text(encoding="utf-8")


def markdown_relative_links(text: str) -> list[str]:
    """Return relative markdown link targets, excluding anchors and URLs."""
    targets: list[str] = []
    for _, target in MARKDOWN_LINK_RE.findall(text):
        if "://" in target or target.startswith("#"):
            continue
        targets.append(target)
    return targets


def required_files_missing(repo_root: Path) -> list[str]:
    """Return required file paths that are missing."""
    required = DOC_FILES + SCRIPT_FILES + [".github/workflows/ci.yml"]
    missing = [item for item in required if not (repo_root / item).exists()]
    return missing


def broken_markdown_links(repo_root: Path, relative_path: str) -> list[str]:
    """Return broken relative links for one markdown file."""
    file_path = repo_root / relative_path
    content = read_text(file_path)
    broken: list[str] = []
    for target in markdown_relative_links(content):
        if not (file_path.parent / target).resolve().exists():
            broken.append(target)
    return broken


def readme_document_links(repo_root: Path) -> list[str]:
    """Return markdown links declared in the README documents section."""
    content = read_text(repo_root / "README.md")
    return markdown_relative_links(content)


def ci_uses_local_wrappers(repo_root: Path) -> bool:
    """Return whether CI runs local lint and test wrappers."""
    content = read_text(repo_root / ".github/workflows/ci.yml")
    return "bin/lint" in content and "bin/test" in content


def has_trailing_whitespace(text: str) -> bool:
    """Return whether any line ends with trailing spaces or tabs."""
    return any(line.rstrip(" \t") != line for line in text.splitlines())


def file_has_tabs(text: str) -> bool:
    """Return whether the file contains hard tab characters."""
    return "\t" in text


def shebang_is_present(text: str) -> bool:
    """Return whether a script starts with a shebang."""
    return text.startswith("#!")
