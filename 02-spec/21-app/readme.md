# Application Domain Specifications (`21-app`)

> **/goal** Maintain clean architectural standards and boundaries for the application specification container (`02-spec/21-app/`).
> **/learn** In downstream projects that build applications, `21-app/` contains product-level feature specifications. In the `coding-guidelines` meta-repository, this directory is strictly a reserved container template and MUST NOT contain meta-guidelines or prompt executions.

---

## 🎯 Container Invariant & Boundary Rules

1. **Application Specifications Reserved Container:**
   `02-spec/21-app/` serves as a reserved container for application-level domain specifications (e.g., photo-editor features in `cat-my`, CLI engines in `gitmap`, markdown editor modules in `letsmarknow`).

2. **Strict Meta-Repository Hygiene (`coding-guidelines`):**
   - **NO Coding Guidelines in `21-app`:** Foundational coding guidelines belong strictly in `02-spec/02-coding-guidelines/`.
   - **NO Prompts in `21-app`:** Autonomous execution prompts belong strictly in `01-prompts/15-cg-execute/` and related folders.
   - **NO Skills in `21-app`:** AI agent skills belong strictly in `.agents/skills/`.
   - In `coding-guidelines`, `02-spec/21-app/` remains empty (holding only `readme.md` and `.gitkeep`) except when an active task is undergoing specification authoring.

3. **Rule 11 Non-Negotiable Boundary (Spec 21 Exclusion):**
   Under Rule 11 of `AGENTS.md`, `02-spec/21-app/` through `02-spec/25-app-spec-audit/` MUST NEVER be synchronized, copied, or touched across repositories. Downstream applications maintain their own independent, isolated product specifications.

---

## Specification Authoring Standards for Application Repositories

When authoring specifications in an application repository:
1. **Pure Spec Authoring Isolation:** Spec creation must focus strictly on architecture, contracts, and requirements (NO-BUILD, NO-TEST, NO-CODE-CHANGES).
2. **Directory Structure:**
   - Single feature: `02-spec/21-app/xx-<feature-slug>.md`
   - Modular feature: `02-spec/21-app/xx-<feature-slug>/` with `01-architecture-spec.md`, `02-component-spec.md`, `03-verification-spec.md`
3. **Lossless Verbatim Capture:** Preserve 100% of user prompt directives under `## User Request (Verbatim)`.
4. **Decoupled Task Planning:** Actionable plans live in `.ai-memory/plans/pending/xx-<slug>.md` and `.ai-memory/plans/subtasks/xx-<slug>/`.

---

## Active Contents

| File / Folder | Title | Type | Status |
| :--- | :--- | :--- | :--- |
| [`.gitkeep`](.gitkeep) | Git Directory Anchor | System | Active |
| [`readme.md`](readme.md) | Application Specification Container Standard & Index | Standard Index | Active |
| [`25-coding-guidelines-and-folder-structure-instructions/`](25-coding-guidelines-and-folder-structure-instructions/) | Coding Guidelines & Folder Structure Instructions (v4 Archival, Spec 21 Sanitization, Multi-Repo Leak Cleanup) | Active Task Spec | Active |
| [`01-create-prompts-for-100-improvements/`](01-create-prompts-for-100-improvements/) | Create Prompts for 100 Improvements (UI Level, Code Base, Whole System) | Task Spec | Active |
| [`02-detail-plans-and-improvements-for-review/`](02-detail-plans-and-improvements-for-review/) | Detail Plans and Improvements for Review (Multi-Step Roadmaps & Review-First Gate) | Task Spec | Active |
| [`03-rename-and-sync-100-improvements-prompts-and-skills/`](03-rename-and-sync-100-improvements-prompts-and-skills/) | Rename and Sync 100 Improvements Prompts and Skills (Suite Taxonomy Upgrade, Deprecation Purge Engine, 43-Repo Fleet Synchronization) | Active Task Spec | Active |
| [`04-next-100-ideas-for-improvements/`](04-next-100-ideas-for-improvements/) | Next 100 Ideas for Improvements Prompt, Skills & Fleet Synchronization | Active Task Spec | Active |
| [`05-improvements-folder-prompts/`](05-improvements-folder-prompts/) | Improvements Folder Prompts: Check Pending & Implement Next 10 Improvements Spec, Skills & Fleet Sync | Active Task Spec | Active |
| [`06-cg-execute-golang-pointers-and-mutation-reduce/`](06-cg-execute-golang-pointers-and-mutation-reduce/) | Golang Pointer Reduction & Mutation Elimination Prompt, Companion Skills & Fleet Sync | Active Task Spec | Active |
| [`03-execute-n-steps-letterly-prompt-formatter/`](03-execute-n-steps-letterly-prompt-formatter/) | Letterly Prompt Formatter — gold standard alignment, mobile one-liner, fleet sync, SQL repository pattern & enum actions (spec 04, 2026-10-23) | Active Task Spec | Active |
