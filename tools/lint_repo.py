"""Repository lint checks for the current docs-first project stage."""

from __future__ import annotations

from pathlib import Path
import sys

from tools import repo_checks


def check_required_files(repo_root: Path) -> list[str]:
    """Return missing required file errors."""
    return [
        f"missing required file: {item}"
        for item in repo_checks.required_files_missing(repo_root)
    ]


def check_markdown_links(repo_root: Path) -> list[str]:
    """Return broken markdown link errors."""
    errors: list[str] = []
    for path in repo_checks.DOC_FILES:
        if not (repo_root / path).exists():
            continue
        for target in repo_checks.broken_markdown_links(repo_root, path):
            errors.append(f"{path}: broken link target {target}")
    return errors


def check_text_hygiene(repo_root: Path) -> list[str]:
    """Return text hygiene errors for docs and scripts."""
    errors: list[str] = []
    for relative_path in repo_checks.DOC_FILES + repo_checks.SCRIPT_FILES:
        if not (repo_root / relative_path).exists():
            continue
        text = repo_checks.read_text(repo_root / relative_path)
        if repo_checks.has_trailing_whitespace(text):
            errors.append(f"{relative_path}: trailing whitespace")
        if repo_checks.file_has_tabs(text):
            errors.append(f"{relative_path}: hard tabs are not allowed")
    return errors


def check_scripts(repo_root: Path) -> list[str]:
    """Return shell script errors."""
    errors: list[str] = []
    for relative_path in ("bin/lint", "bin/test"):
        script = repo_root / relative_path
        if not script.exists():
            continue
        text = repo_checks.read_text(script)
        if not repo_checks.shebang_is_present(text):
            errors.append(f"{relative_path}: missing shebang")
        if not script.stat().st_mode & 0o111:
            errors.append(f"{relative_path}: script is not executable")
    return errors


def check_ci(repo_root: Path) -> list[str]:
    """Return CI wiring errors."""
    if not (repo_root / ".github/workflows/ci.yml").exists():
        return []
    if repo_checks.ci_uses_local_wrappers(repo_root):
        return []
    return [".github/workflows/ci.yml: must run bin/lint and bin/test"]


def run(repo_root: Path) -> list[str]:
    """Run all lint checks and return any failures."""
    errors: list[str] = []
    errors.extend(check_required_files(repo_root))
    errors.extend(check_markdown_links(repo_root))
    errors.extend(check_text_hygiene(repo_root))
    errors.extend(check_scripts(repo_root))
    errors.extend(check_ci(repo_root))
    return errors


def main() -> int:
    """Run lint checks and print a concise report."""
    repo_root = Path(__file__).resolve().parent.parent
    errors = run(repo_root)
    if not errors:
        print("lint: ok")
        return 0

    for error in errors:
        print(f"lint: {error}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
