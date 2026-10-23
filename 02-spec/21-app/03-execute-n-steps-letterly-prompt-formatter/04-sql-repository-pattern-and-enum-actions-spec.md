# 04 — SQL Repository Pattern & Enum-Driven Actions (prompt instruction + skills)

- parent spec folder: `02-spec/21-app/03-execute-n-steps-letterly-prompt-formatter/`
- parent plan: `.ai-memory/plans/pending/03-execute-n-steps-letterly-prompt-formatter.md`
- date: 2026-10-23
- sequence note: this slug's spec/plan/subtasks live at sequence `03` (Phase 1 completed in a prior pass). `gitmap spec next` returns 26 for a brand-new spec; reusing 03 for this slug is deliberate — do not mint a duplicate.

## User Request (Verbatim)

# Execute N-Steps — Letterly Prompt Formatter: high priority instruction, non-negotiable task

For the prompts, see the execute folder. Can we please add new prompt instruction and skills for agent cursor? For the SQL should be following the repository pattern regardless of the programming language, and every action should be coming from the enum, so that it can be switched, checked. And enum's definition, how we use it, that should be defined properly, how it should be written. And also it should have the util methods checkup. And make sure that from the coding guideline and error manage, if we have something new to add or can we fine-tune prompts and skills, please confirm that. Do not need to confirm, just update and improve. Finally, let me know your final updates. Is it understood? And make sure everything is committed for all the repositories when the work is done. Is it clear? Make sure you commit, push and release at the end.

## task: Execute N-Steps — Letterly Prompt Formatter
## slug: execute-n-steps-letterly-prompt-formatter

# Actionable Items Must Follow Non-Negotiable

1. Git pull BEFORE starting any work — first try `gitmap pull <repo path>` (or run `gitmap pull` in repository folder); if that fails, fall back to regular `git pull` and resolve any merge conflicts so the repository is completely up to date and clean before starting.
2. Write spec under 02-spec/21-app/XX-execute-n-steps-letterly-prompt-formatter/ and enqueue plan task in .ai-memory/plans/pending/XX-execute-n-steps-letterly-prompt-formatter.md (subtasks in .ai-memory/plans/subtasks/XX-execute-n-steps-letterly-prompt-formatter/) first, (xx should be figured out by the folder structure and running AI tools, xx is sequence 01, 02)
3. Search codebase exclusively via GitMap (gitmap aum search, gitmap find, gitmap cat, gitmap ps, gitmap py, gitmap llm train); TOTAL BAN on rg, ripgrep, grep, git grep, Select-String
4. Strictly use relative Git paths (02-spec/..., .ai-memory/..., cmd/...); only add the relative paths, never add the absolute path during your work, and ensure this is respected on the release page and in release notes as well
5. Add new prompt instruction and skills for agent cursor in the execute folder.
6. Ensure SQL follows the repository pattern regardless of the programming language.
7. Ensure every action comes from the enum, allowing it to be switched and checked.
8. Define enum's definition, usage, and writing properly, including util methods checkup.
9. Update and improve coding guidelines and error management without confirmation.
10. Commit, push, and release all changes for all repositories when work is done.

## Must follow and spawn agent using

[execute-parent-task-with-n-steps-v6](file;.agents/skills/execute-parent-task-with-n-steps-v6)

## Additional Instructions

- [/plan](slashCommand;plan) first before doing work for improved work or less friction, faster execution for AI and coding without errors and following Coding Guidelines.
- [/learn](slashCommand;learn) from [gitmap](file;.agents/skills/gitmap) skill to leverage GitMap high-speed search, toolchain discovery, and caching.
- Track in-flight file touches and prevent write collisions using GitMap RC Heatmap (`gitmap rc heatmap status` / `gitmap rc heatmap tree` / `gitmap rc heatmap live` / `gitmap rc heatmap sync`).
- Only add relative paths, never add absolute paths during work; this must be respected on release page and in release notes as well.

## The rules to codify (normative content)

### A. SQL follows the repository pattern — every language, no exceptions

1. All SQL and data access lives behind a repository interface: one repository per aggregate or table group. Services, handlers, and use-cases NEVER embed SQL strings, query builders, or ORM calls directly.
2. Repository methods are intention-named (`FindByID`, `ListActive`, `Insert`, `Update`, `SoftDelete`) — never `RunQuery`, `ExecuteRaw`, or stringly-typed table/column parameters from callers.
3. Dialect and driver specifics stay inside the repository implementation; swapping the database means swapping the implementation, never the callers.
4. Multi-repository transactions go through a unit-of-work abstraction; no ad-hoc transaction handling scattered in services.
5. The rule is language-agnostic: Go, TypeScript, Python, Rust, PHP — the shape is identical; only the idiom changes.

### B. Every action comes from an enum — switchable and checkable

1. Every action, operation, command kind, and event kind is a member of a named enum. Dispatch happens by switching/matching on the enum — never by comparing raw strings or magic numbers.
2. Switches over action enums are exhaustive: the compiler or linter must flag a missing case. The default branch raises a typed error (see D) — never a silent no-op.
3. Before dispatch, the action is checked (`IsValid` / membership check from the enum's utils). Unchecked external input never reaches the switch.
4. Adding an action = adding one enum member + one switch case. No other registration step may be required.

### C. Enum authoring — definition, usage, writing, util-methods checkup

Definition rules:
1. One enum per concept, defined in the shared types/constants module of the repo (never inline in a handler or test).
2. Type and members in the language's idiomatic casing; serialized enums carry explicit, stable string values that never change once shipped.
3. Where the language needs it, an explicit unknown/zero sentinel exists and is never a valid action.

Usage rules:
4. No raw literals anywhere an enum exists — comparisons, map keys, logs, and payloads all use the member or its canonical string value.
5. External input enters ONLY through the enum's parse util, which returns the member or a typed error.

Util-methods checkup (every enum ships these, or a documented exception):
6. `String()` / label — canonical string form of a member.
7. `Parse` / `TryParse` — string → member, typed error on unknown input.
8. `IsValid` — membership check used before dispatch.
9. `All` / `Values` — the full member list (drives exhaustive tests and UI lists).
10. Checkup: any enum missing a util gets the util added in the same change that touches the enum — never deferred.

### D. Error-management tie-ins

1. Unknown enum value, failed parse, or unhandled switch case → typed error (the repo's AppError/result convention) with the offending value in the message; never a panic, never a silent default.
2. Repository errors are wrapped with context at the repository boundary (which repository, which operation) before crossing into services.
3. Error prompts/skills must show the enum-parse failure and repository-wrap patterns as the canonical examples.

## Where it lands

- Letterly gold standard: `01-prompts/22-letterly/03-execute-n-steps-letterly.md` gains a mandatory instruction block carrying A–D (the generated prompts must include it), mirrored in `01-prompts/23-cursor-prompts/03-execute-n-steps-letterly-cursor.md`.
- Fine-tune (no confirmation): coding-guideline prompts (`01-prompts/05-coding-guidelines/`), error-management prompts, and the enum/database/error skills — align wording with A–D wherever they touch SQL, actions, or enums.
- New skill (agents + Cursor mirrors): codifies A–D as an actionable checklist with the util-methods checkup.
- Finish: `gitmap sync` fleet-wide; every touched repo committed + pushed; coding-guidelines-v24 minor bump + release; relative paths only, including release notes.

## Acceptance

- The instruction block is present in both Letterly execute prompts (agents + Cursor) and reads as a non-negotiable in the generated output.
- The new skill exists in `.agents/skills/` and `.cursor/skills/` with the util-methods checkup as a checklist.
- Enum/db/error skills and coding-guideline/error prompts show aligned A–D wording (spot-check evidence in ledgers).
- Fleet sync done; `gitmap status --dirty` clean across repos; release tag on origin.
