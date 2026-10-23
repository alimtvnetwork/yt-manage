# Specification 02: Mobile One-Liner Streamlining & Skills Update

## 1. System Overview & Problem Statement

Mobile execution environments require ultra-lean, dense, single-paragraph prompts designed for on-the-go input via speech-to-text without desktop multi-line friction. The user explicitly directed:
> "But mobile prompt needs to run, remember one liner. Remember that. Do not need to have too much things for the mobile runner. Based on that, update the plan letterly, release letterly, CI/CD, and other stuff. Fix all of these following that. Second, update the skills for the agent cursor and any other skills that is there."

```mermaid
flowchart LR
    A["Raw STT Audio Transcript"] --> B["Mobile Formatter Engine"]
    B --> C["Strict Single-Line Prompt (0 Line Breaks)"]
    C --> D["Fast Mobile Agent Runner"]
```

## 2. Mobile One-Liner Structural Blueprint

`01-prompts/22-letterly/01-mobile-letterly.md` and `01-prompts/23-cursor-prompts/01-mobile-letterly-cursor.md` enforce:
1. **Zero Newlines (Single Continuous Line):** The generated output MUST be exactly one continuous paragraph with 0 newline characters (`\n`).
2. **Dense Component Assembly:**
   - Slash Command Trigger: `[/goal](slashCommand;goal) [/learn](slashCommand;learn) from [gitmap](file;.agents/skills/gitmap) [/plan](slashCommand;plan) `
   - Title & Metadata: `# <task title>: high priority instruction, non-negotiable task (task: <task title>, slug: <task-slug>): `
   - Cleaned Transcript: `${Input Text Verbatim}; `
   - Non-Negotiable Directives: `git pull before starting (gitmap pull fallback git pull); write spec under 02-spec/21-app/XX-<slug>/ and enqueue plan in .ai-memory/plans/pending/XX-<slug>.md (subtasks in .ai-memory/plans/subtasks/XX-<slug>/) first (xx is sequence 01, 02); planning is strictly dedicated to architecture and decoupled from execution (no-build, no-test, no-code-changes); search codebase exclusively via gitmap; track touches using gitmap rc heatmap; strictly use relative git paths only; `
   - Discrete Tasks: `tasks: 1. [task-1], 2. [task-2], 3. [task-3]; `
   - Closing Skill Invocation: `strictly use relative git paths only; must follow and spawn agent using [execute-parent-task-with-n-steps-v6](file;.agents/skills/execute-parent-task-with-n-steps-v6) - [/plan](slashCommand;plan) first before doing work for improved work or less friction, faster work and more pure and effective work.`

## 3. Skills Update Across `.agents/skills/` & `.cursor/skills/`

The updated gold standard conventions must be reflected across agent skills:

### 1. `execute-parent-task-with-n-steps-v6/skill.md`:
- Re-anchor plan motto under Additional Instructions to:
  `[/plan](slashCommand;plan) first before doing work for improved work or less friction, faster work and more pure and effective work.`
- Update Phase 1 Spec breakdown payload to reference:
  `Write spec under 02-spec/21-app/XX-<slug>/ and enqueue plan task in .ai-memory/plans/pending/XX-<slug>.md (subtasks in .ai-memory/plans/subtasks/XX-<slug>/) first, (xx should be figured out by the folder structure and running AI tools, xx is sequence 01, 02)`

### 2. `plan-spec-steps-v2/skill.md`:
- Ensure planning directive aligns with the decoupled architecture motto: "for improved work or less friction, faster work and more pure and effective work".

### 3. Cursor Skills Mirrors (`.cursor/skills/`):
- Synchronize all changes into `.cursor/skills/` ensuring IDE markdown link syntax `(file;.cursor/skills/<skill>)`.

## 4. Verification Protocol

1. `01-mobile-letterly.md` output format is tested to guarantee 0 newline characters.
2. `.agents/skills/execute-parent-task-with-n-steps-v6/skill.md` contains the expanded motto.
3. `.cursor/skills/execute-parent-task-with-n-steps-v6/skill.md` contains the expanded motto.
