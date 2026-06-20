"""Run the test suite under stdlib tracing and enforce a coverage floor."""

from __future__ import annotations

from importlib import util
from pathlib import Path
import ast
import sys
import trace
import types
import unittest


TARGET_MODULES = [
    "tools.repo_checks",
    "tools.lint_repo",
]


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


def coverage_summary(repo_root: Path, tracer: trace.Trace) -> tuple[float, list[str]]:
    """Return total coverage and per-module human-readable rows."""
    results = tracer.results()
    counts = results.counts
    rows: list[str] = []
    total_executable = 0
    total_hit = 0

    for module_name in TARGET_MODULES:
        path = module_path(repo_root, module_name)
        executable = executable_lines(path)
        hit = sum(1 for line in executable if counts.get((str(path), line), 0) > 0)
        total_executable += len(executable)
        total_hit += hit
        percent = 100.0 if not executable else (hit / len(executable)) * 100.0
        rows.append(f"{module_name}: {hit}/{len(executable)} lines ({percent:.1f}%)")

    total = 100.0 if not total_executable else (total_hit / total_executable) * 100.0
    return total, rows


def main(argv: list[str] | None = None) -> int:
    """Run the suite and fail if total coverage drops below the threshold."""
    args = argv or sys.argv[1:]
    minimum = float(args[0]) if args else 90.0
    repo_root = Path(__file__).resolve().parent.parent

    result, tracer = run_suite_with_trace(repo_root)
    if not result.wasSuccessful():
        return 1

    total, rows = coverage_summary(repo_root, tracer)
    for row in rows:
        print(row)
    print(f"total: {total:.1f}%")

    if total < minimum:
        print(
            f"coverage gate failed: total {total:.1f}% is below required {minimum:.1f}%",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
