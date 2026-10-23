#!/usr/bin/env python3
"""Mechanical validator for the desktop Letterly formatter output shape.

The Letterly/Muse prompts describe the required output in prose, and prose is
lossy: the same misreadings recur (fenced output, literal "High Priority
Instruction" as the title, slug on the wrong line / in the wrong case). This
script enforces the shape mechanically instead of hoping the formatter got it
right. It is the executable counterpart of
``01-prompts/22-letterly/02-desktop-letterly.md`` ("Output Format").

Checks, in order:
  1. No fenced code blocks anywhere (``` fence is forbidden, even ```markdown).
  2. First content line is the single-line title:
     ``# <task title>: high priority instruction, non-negotiable task``
     where <task title> is derived from the input subject and is NEVER the
     literal words "High Priority Instruction".
  3. Task & Slug blocks:
     - ``## task: <task title>`` — human-readable task subject
     - ``## slug: <task-slug>`` — lowercase-hyphenated slug
       (never ``high-priority-instruction``, never "High Priority Instruction").
  4. Slot order: title < verbatim input < ``## task:`` < ``## slug:`` <
     ``# Actionable Items`` < ``## Must follow`` < ``## Additional Instructions``.
  5. Fixed actionable items 1-7 present verbatim (item 1 checks pull first,
     item 2 allows a substituted slug in place of the ``<slug>`` placeholder).
  6. Mandatory agent-invocation suffix link present.
  7. ``## Additional Instructions`` carries the /plan and /learn directives.

Usage:
    gitmap py linter-scripts/check-letterly-format.py <formatted-output.md>
    gitmap py linter-scripts/check-letterly-format.py --stdin < formatted-output.md

Exit code 0 when the output conforms, 1 otherwise (violations on stdout).
"""

from __future__ import annotations

import re
import sys

TITLE_RE = re.compile(r"^# (.+): high priority instruction, non-negotiable task$")
FENCE_RE = re.compile(r"^\s*```")
ACTIONABLE_HEADER = "# Actionable Items Must Follow Non-Negotiable"
MUST_FOLLOW_HEADER = "## Must follow and spawn agent using"
ADDITIONAL_HEADER = "## Additional Instructions"
AGENT_LINK = "[execute-parent-task-with-n-steps-v6](file;.agents/skills/execute-parent-task-with-n-steps-v6)"

FIXED_ITEM_PREFIXES = [
    "1. Git pull BEFORE starting any work — first try `gitmap pull",
    "2. Write spec under 02-spec/21-app/",
    "3. Search codebase exclusively via GitMap (gitmap aum search, gitmap find, gitmap cat, gitmap ps, gitmap py, gitmap llm train); TOTAL BAN on rg, ripgrep, grep, git grep, Select-String",
    "4. Strictly use relative Git paths (02-spec/..., .ai-memory/..., cmd/...); only add the relative paths, never add the absolute path during your work, and ensure this is respected on the release page and in release notes as well",
    "5. Use `gitmap` AI agents to enter data",
    "6. Task completion includes committing and pushing to Git",
    "7. Give every Task-NN in the confirmed task breakdown a `Slug:` sub-item",
]


def _nonempty_lines(text):
    return [(i + 1, line) for i, line in enumerate(text.splitlines()) if line.strip()]


def check_no_fences(numbered):
    violations = []
    for lineno, line in numbered:
        if FENCE_RE.match(line):
            violations.append(f"line {lineno}: fenced code block is forbidden (found `{line.strip()}`)")
    return violations


def check_title(numbered):
    violations = []
    if not numbered:
        return ["empty output: missing title line"]
    lineno, first = numbered[0]
    match = TITLE_RE.match(first.strip())
    if not match:
        return [f"line {lineno}: first line must be `# <task title>: high priority instruction, non-negotiable task`"]
    title = match.group(1).strip()
    if not title:
        violations.append(f"line {lineno}: task title is empty")
    if title.lower() == "high priority instruction":
        violations.append(
            f"line {lineno}: title must be derived from the input subject, "
            'never the literal words "High Priority Instruction"'
        )
    return violations


def _find_header(numbered, header):
    for lineno, line in numbered:
        if line.strip() == header:
            return lineno
    return None


def check_task_and_slug_blocks(numbered):
    violations = []
    task_lineno = None
    slug_lineno = None
    task_value = None
    slug_value = None
    for lineno, line in numbered:
        stripped = line.strip()
        if stripped.startswith("## task:"):
            task_lineno = lineno
            task_value = stripped[len("## task:"):].strip()
        elif stripped == "## task":
            violations.append(f"line {lineno}: `## task:` must include the task title (`## task: <task title>`)")
            task_lineno = lineno
        elif stripped.startswith("## slug:"):
            slug_lineno = lineno
            slug_value = stripped[len("## slug:"):].strip()
        elif stripped == "## slug":
            violations.append(f"line {lineno}: use `## slug: <task-slug>` format on a single line")
            slug_lineno = lineno
        elif re.match(r"^slug\s*:", stripped):
            violations.append(
                f"line {lineno}: old `slug: <value>` without `##` is forbidden; "
                "use `## slug: <task-slug>`"
            )

    if task_lineno is None:
        violations.append("missing `## task: <task title>` subheader")
    else:
        if not task_value:
            violations.append(f"line {task_lineno}: task title value is empty")
        elif task_value.lower() == "high priority instruction":
            violations.append(
                f"line {task_lineno}: task title must be derived from input subject, "
                'never the literal words "High Priority Instruction"'
            )

    if slug_lineno is None:
        violations.append("missing `## slug: <task-slug>` subheader")
    else:
        if not slug_value:
            violations.append(f"line {slug_lineno}: slug value is empty")
        elif slug_value.lower() == "high priority instruction":
            violations.append(
                f"line {slug_lineno}: slug must never be literal words \"High Priority Instruction\""
            )
        elif slug_value == "high-priority-instruction":
            violations.append(
                f"line {slug_lineno}: slug must be derived from task subject, never `high-priority-instruction`"
            )
        elif not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", slug_value):
            violations.append(
                f"line {slug_lineno}: slug value must be lowercase-hyphenated (found `{slug_value}`)"
            )

    return violations, task_lineno, slug_lineno


def check_slot_order(numbered, task_lineno, slug_lineno):
    violations = []
    title_lineno = numbered[0][0] if numbered else None
    actionable = _find_header(numbered, ACTIONABLE_HEADER)
    must_follow = _find_header(numbered, MUST_FOLLOW_HEADER)
    additional = _find_header(numbered, ADDITIONAL_HEADER)
    for name, lineno in [
        ("`# Actionable Items Must Follow Non-Negotiable`", actionable),
        ("`## Must follow and spawn agent using`", must_follow),
        ("`## Additional Instructions`", additional),
    ]:
        if lineno is None:
            violations.append(f"missing {name} section")
    ordered = [
        ("title line", title_lineno),
        ("`## task:`", task_lineno),
        ("`## slug:`", slug_lineno),
        ("`# Actionable Items`", actionable),
        ("`## Must follow`", must_follow),
        ("`## Additional Instructions`", additional),
    ]
    present = [(name, n) for name, n in ordered if n is not None]
    for (prev_name, prev_n), (name, n) in zip(present, present[1:]):
        if n < prev_n:
            violations.append(f"slot order violated: {name} (line {n}) appears before {prev_name} (line {prev_n})")
    first_sub = task_lineno if task_lineno is not None else slug_lineno
    if title_lineno is not None and first_sub is not None:
        between = [
            line for n, line in numbered
            if title_lineno < n < first_sub
            and line.strip() and not line.strip().startswith("#")
        ]
        if not between:
            violations.append(
                f"`## task:` / `## slug:` (line {first_sub}) must come after the verbatim input, "
                f"not directly after the title line (line {title_lineno})"
            )
    return violations


def check_fixed_items(numbered, actionable_lineno):
    violations = []
    if actionable_lineno is None:
        return violations
    items = {}
    for lineno, line in numbered:
        if lineno <= actionable_lineno:
            continue
        stripped = line.strip()
        m = re.match(r"^([1-7])\.\s", stripped)
        if m:
            items[int(m.group(1))] = (lineno, stripped)
        if stripped.startswith("## "):
            break
    for idx, prefix in enumerate(FIXED_ITEM_PREFIXES, start=1):
        if idx not in items:
            violations.append(f"fixed actionable item {idx} is missing")
            continue
        lineno, text = items[idx]
        if idx == 1:
            if not text.startswith(prefix) or "resolve any merge conflicts" not in text:
                violations.append(f"line {lineno}: fixed item 1 text was modified")
        elif idx == 2:
            if not text.startswith(prefix) or "enqueue plan task in .ai-memory/plans/" not in text:
                violations.append(f"line {lineno}: fixed item 2 text was modified")
        elif not text.startswith(prefix):
            violations.append(f"line {lineno}: fixed item {idx} text was modified")
    return violations


def check_suffix_and_instructions(text):
    violations = []
    if AGENT_LINK not in text:
        violations.append("missing mandatory agent-invocation suffix link (execute-parent-task-with-n-steps-v6)")
    if "[/plan](slashCommand;plan)" not in text:
        violations.append("`## Additional Instructions` is missing the /plan directive")
    if "[/learn](slashCommand;learn)" not in text:
        violations.append("`## Additional Instructions` is missing the /learn directive")
    return violations


def validate(text):
    numbered = _nonempty_lines(text)
    violations = []
    violations.extend(check_no_fences(numbered))
    violations.extend(check_title(numbered))
    slug_violations, task_lineno, slug_lineno = check_task_and_slug_blocks(numbered)
    violations.extend(slug_violations)
    violations.extend(check_slot_order(numbered, task_lineno, slug_lineno))
    actionable_lineno = _find_header(numbered, ACTIONABLE_HEADER)
    violations.extend(check_fixed_items(numbered, actionable_lineno))
    violations.extend(check_suffix_and_instructions(text))
    return violations


def main(argv):
    if "--stdin" in argv:
        text = sys.stdin.read()
    elif len(argv) == 2:
        with open(argv[1], "r", encoding="utf-8") as fh:
            text = fh.read()
    else:
        print(__doc__.strip().splitlines()[-6], file=sys.stderr)
        print("usage: check-letterly-format.py <file> | --stdin", file=sys.stderr)
        return 2
    violations = validate(text)
    if not violations:
        print("OK: Letterly output conforms to the required format.")
        return 0
    print(f"FAIL: {len(violations)} violation(s):")
    for v in violations:
        print(f"  - {v}")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
