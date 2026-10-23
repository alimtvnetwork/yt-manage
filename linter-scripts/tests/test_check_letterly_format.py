"""Golden-file tests for ``linter-scripts/check-letterly-format.py``.

Each fixture is a formatter output; the test asserts the validator's verdict.
The FAIL fixtures are the exact misreadings observed in production on
2026-10-09, preserved here as regression tests:

  * ``FENCED`` — output wrapped in a ```markdown fence.
  * ``LITERAL_TITLE`` — title line used the literal words "High Priority
    Instruction" instead of the derived task subject.
  * ``OLD_SLUG_LINE`` — legacy single-line ``slug: <value>`` form.
  * ``SLUG_BEFORE_INPUT`` — slug block placed before the verbatim input.
  * ``MISSING_ITEM`` — fixed actionable item 4 deleted.

Tests are black-box: fixtures are written to temp files and the script is
invoked as a subprocess (exit code + output asserted).
"""
from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "linter-scripts" / "check-letterly-format.py"

VALID = """# SEO Writing and Folder Structure Instructions: high priority instruction, non-negotiable task

Hi there. When you started with the SEO writing, before that there was only one or two article write. Follow that article writing pattern.

## task: SEO Writing and Folder Structure Instructions
## slug: seo-writing-and-folder-structure-instructions

# Actionable Items Must Follow Non-Negotiable

1. Git pull BEFORE starting any work — first try `gitmap pull <repo path>` (or run `gitmap pull` in repository folder); if that fails, fall back to regular `git pull` and resolve any merge conflicts so the repository is completely up to date and clean before starting.
2. Write spec under 02-spec/21-app/<slug>/ and enqueue plan task in .ai-memory/plans/<slug>.md (subtasks in .ai-memory/plans/subtasks/<slug>/) first
3. Search codebase exclusively via GitMap (gitmap aum search, gitmap find, gitmap cat, gitmap ps, gitmap py, gitmap llm train); TOTAL BAN on rg, ripgrep, grep, git grep, Select-String
4. Strictly use relative Git paths (02-spec/..., .ai-memory/..., cmd/...); only add the relative paths, never add the absolute path during your work, and ensure this is respected on the release page and in release notes as well
5. Use `gitmap` AI agents to enter data
6. Task completion includes committing and pushing to Git
7. Give every Task-NN in the confirmed task breakdown a `Slug:` sub-item derived from the root slug (`<root-slug> - Task NN`, e.g. `Slug: SEO Writing and Folder Structure Instructions - Task 01`) so GitMap can create and verify subtasks under the root task
8. Follow the existing article writing pattern from the first one or two articles

## Must follow and spawn agent using

[execute-parent-task-with-n-steps-v6](file;.agents/skills/execute-parent-task-with-n-steps-v6)

## Additional Instructions

- [/plan](slashCommand;plan) first before doing work for improved work or less friction, faster work and more pure and effective work.
- [/learn](slashCommand;learn) from [gitmap](file;.agents/skills/gitmap) skill to leverage GitMap high-speed search, toolchain discovery, and caching.
- Only add the relative paths, never add the absolute path during your work; this should be respected on the release page and in release notes as well.
"""

FENCED = "```markdown\n" + VALID + "```\n"

LITERAL_TITLE = VALID.replace(
    "# SEO Writing and Folder Structure Instructions: high priority instruction, non-negotiable task",
    "# High Priority Instruction: high priority instruction, non-negotiable task",
    1,
).replace(
    "## task: SEO Writing and Folder Structure Instructions\n## slug: seo-writing-and-folder-structure-instructions",
    "## task: High Priority Instruction\n## slug: high-priority-instruction",
    1,
)

OLD_SLUG_LINE = VALID.replace(
    "## task: SEO Writing and Folder Structure Instructions\n## slug: seo-writing-and-folder-structure-instructions",
    "slug: seo-writing-and-folder-structure-instructions",
    1,
)

SLUG_BEFORE_INPUT = VALID.replace(
    """Hi there. When you started with the SEO writing, before that there was only one or two article write. Follow that article writing pattern.

## task: SEO Writing and Folder Structure Instructions
## slug: seo-writing-and-folder-structure-instructions
""",
    """## task: SEO Writing and Folder Structure Instructions
## slug: seo-writing-and-folder-structure-instructions

Hi there. When you started with the SEO writing, before that there was only one or two article write. Follow that article writing pattern.
""",
    1,
)

MISSING_ITEM = "\n".join(
    line for line in VALID.splitlines()
    if not line.startswith("5. Use `gitmap` AI agents to enter data")
) + "\n"

MISSING_TASK_HEADER = VALID.replace("## task: SEO Writing and Folder Structure Instructions\n", "", 1)

UPPERCASE_SLUG = VALID.replace(
    "## slug: seo-writing-and-folder-structure-instructions",
    "## slug: SEO-Writing-And-Folder-Structure-Instructions",
    1,
)


def run_validator(body: str) -> subprocess.CompletedProcess:
    with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as fh:
        fh.write(body)
        path = fh.name
    return subprocess.run(
        [sys.executable, str(SCRIPT), path],
        capture_output=True,
        text=True,
    )


class TestCheckLetterlyFormat(unittest.TestCase):
    def test_valid_output_passes(self):
        proc = run_validator(VALID)
        self.assertEqual(proc.returncode, 0, proc.stdout)

    def test_fenced_output_fails(self):
        proc = run_validator(FENCED)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("fenced code block is forbidden", proc.stdout)

    def test_literal_title_fails(self):
        proc = run_validator(LITERAL_TITLE)
        self.assertEqual(proc.returncode, 1)
        self.assertIn('never the literal words "High Priority Instruction"', proc.stdout)

    def test_old_slug_line_fails(self):
        proc = run_validator(OLD_SLUG_LINE)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("old `slug: <value>` without `##` is forbidden", proc.stdout)

    def test_slug_before_input_fails(self):
        proc = run_validator(SLUG_BEFORE_INPUT)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("must come after the verbatim input", proc.stdout)

    def test_missing_fixed_item_fails(self):
        proc = run_validator(MISSING_ITEM)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("fixed actionable item 5 is missing", proc.stdout)

    def test_missing_task_header_fails(self):
        proc = run_validator(MISSING_TASK_HEADER)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("missing `## task: <task title>` subheader", proc.stdout)

    def test_uppercase_slug_fails(self):
        proc = run_validator(UPPERCASE_SLUG)
        self.assertEqual(proc.returncode, 1)
        self.assertIn("slug value must be lowercase-hyphenated", proc.stdout)


if __name__ == "__main__":
    unittest.main()
