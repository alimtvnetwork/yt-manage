# Specification 02: Spec 21 Sanitization & Guidelines Realignment

## 1. System Overview & Problem Statement

In the Prompt Architect meta-repository architecture, `02-spec/21-app/` serves as a reserved container for application-level domain specifications (e.g. e-commerce engines, photo-editor modules, CLI tools, UI dashboards). It was NEVER intended to house meta-coding guidelines, prompt executions, or transient AI milestones.

Due to legacy prompt phrasing ("Write spec under 02-spec/21-app/XX-<slug>/"), multiple automated AI sessions erroneously authored guideline milestone folders under `02-spec/21-app/`, resulting in 44 misplaced subdirectories and 7 root files.

```mermaid
flowchart TD
    subgraph Misplaced Clutter in 02-spec/21-app
        M1["01-variadic-spread-params-..."]
        M2["02-string-normalization-..."]
        M3["03-guard-clauses-..."]
        M4["41 other milestone folders"]
        M5["7 loose markdown files"]
    end

    subgraph Canonical Homes (Preserved)
        C1["02-spec/02-coding-guidelines/ (Complete)"]
        C2["01-prompts/15-cg-execute/ (Complete)"]
        C3[".agents/skills/ (Complete)"]
    end

    M1 & M2 & M3 & M4 & M5 -->|Audit & Confirm Canonical Presence| C1 & C2 & C3
    M1 & M2 & M3 & M4 & M5 -->|Surgical Purge| DEL["Deleted from 02-spec/21-app/"]
    DEL --> CLEAN["Clean Container: 02-spec/21-app/readme.md + .gitkeep"]
```

## 2. Inventory of Misplaced Folders & Files to Purge

The following 44 directories and 7 loose root files in `02-spec/21-app/` are ephemeral artifacts that duplicate canonical specifications:

### Misplaced Directories (44 to delete):
1. `01-variadic-spread-params-and-multi-repo/`
2. `02-string-normalization-and-equalfoldany/`
3. `03-guard-clauses-and-discrete-conditions/`
4. `04-spread-params-and-pointer-reduction/`
5. `05-repo-cache-and-script-standardization/`
6. `06-gitmap-high-speed-commands-migration/`
7. `07-coding-guideline-actionable-checklist-and-acceptance/`
8. `08-coding-guidelines-verification-and-ci-cd-enforcement/`
9. `09-repo-clean-caches-and-toolchain-hygiene/`
10. `10-gitmap-split-db-schema-and-casbin-rbac/`
11. `11-database-conventions-and-app-architecture/`
12. `12-atomic-commit-checkpoints-and-completion-invariants/`
13. `13-git-reconcile-and-resolve-conflict/`
14. `14-gitmap-macro-automation-engine/`
15. `15-gitmap-ssh-cluster-fleet/`
16. `16-spec-reverse-engineering/`
17. `17-gitmap-pipeline-ai-and-diagnostics/`
18. `18-smart-test-runner-and-inventory/`
19. `19-nuclear-package-modularization/`
20. `20-zero-storage-github-actions-mandate/`
21. `21-ai-verification-retrospective-audit/`
22. `22-spec-authoring-and-validation/`
23. `23-ai-memory-consolidate-reduce/`
24. `24-coding-guideline-and-prompt-execution-improvement/` (completed legacy run)
25. `coding-guideline-and-prompt-execution-improvement/` (unprefixed duplicate)
26. `cd-execute-prompt-suite-v6-migration/`
27. `cg-boolean-and-naming-cleanup/`
28. `cg-branch-immutability-and-clean-construction/`
29. `cg-clean-repo-build-and-caches-v6/`
30. `cg-code-dryness-and-library-extraction-v6/`
31. `cg-execute-folder-02-to-07-standardization/`
32. `cg-execute-folder-08-to-14-standardization/`
33. `cg-execute-folder-15-to-21-standardization/`
34. `cg-execute-folder-22-to-28-standardization/`
35. `cg-execute-folder-29-to-35-standardization/`
36. `cg-execute-folder-36-to-40-standardization/`
37. `cg-file-size-and-function-reduction-v6/`
38. `cg-map-dispatch-and-handler-lookup-v6/`
39. `cg-string-efficiency-v6/`
40. `mime-and-media-type-enums-and-clipboard-data/`
41. `nuclear-package-modularization-and-unit-test-optimization-v6/`
42. `result-wrapper-and-apperror-returns-v6/`
43. `style-guidelines-vertical-spacing-v6/`
44. `subagent-executor-alignment-and-anti-stall-timeout/`

### Misplaced Root Files (7 to delete):
- `01-architecture-spec.md`
- `02-component-and-examples-spec.md`
- `03-migration-and-verification-spec.md`
- `checkpoints.md`
- `findings.md`
- `spec-21-sanitization-plan.md`
- `validation-matrix.md`

### Files to Retain:
- `.gitkeep`
- `readme.md` (rewritten as canonical container documentation)
- `25-coding-guidelines-and-folder-structure-instructions/` (the active task specification directory)

## 3. Canonical Homes Verification

Every rule represented in the purged directories is fully documented in canonical locations:

| Rule Family | Canonical Spec | Canonical Execution Prompt | Canonical Skill |
| :--- | :--- | :--- | :--- |
| Variadic & Spread Params | `02-spec/02-coding-guidelines/34-variadic-and-spread-parameters.md` | `01-prompts/15-cg-execute/34-variadic-and-spread-parameters.md` | `cg-variadic-and-spread-parameters` |
| Map Dispatch | `02-spec/02-coding-guidelines/35-map-dispatch-and-handler-lookup.md` | `01-prompts/15-cg-execute/35-map-dispatch-and-handler-lookup.md` | `cg-map-dispatch-and-handler-lookup` |
| Multi-Language Enums | `02-spec/02-coding-guidelines/36-multi-language-enums-and-traits.md` | `01-prompts/15-cg-execute/36-multi-language-enums-and-traits.md` | `multi-language-enums-and-traits` |
| String Normalization | `02-spec/02-coding-guidelines/37-string-normalization-and-equalfoldany.md` | `01-prompts/15-cg-execute/37-string-normalization-and-equalfoldany.md` | `cg-string-normalization-and-equalfoldany` |
| Pointer Reduction | `02-spec/02-coding-guidelines/38-golang-pointer-reduction-and-value-semantics.md` | `01-prompts/15-cg-execute/38-golang-pointer-reduction-and-value-semantics.md` | `cg-golang-pointer-reduction-and-value-semantics` |
| Split-DB Schema | `02-spec/02-coding-guidelines/17-split-sqlite-databases.md` | `01-prompts/15-cg-execute/10-gitmap-split-db-schema-and-casbin-rbac.md` | `gitmap-split-db-engine` |

## 4. Canonical Container Structure for `02-spec/21-app/`

`02-spec/21-app/readme.md` will define the structural invariant:
- `02-spec/21-app/` is strictly a container for application domain specifications in projects that build software applications (e.g. web apps, desktop apps, microservices).
- In the `coding-guidelines` meta-repository, this folder remains empty (containing only `readme.md` and `.gitkeep`) except when an active task is undergoing authoring.
- Coding guideline rules, prompts, and skills MUST NEVER be placed in `21-app`.

## 5. Verification Protocol

1. After sanitization, `gitmap lf 02-spec/21-app` lists strictly `readme.md`, `.gitkeep`, and `25-coding-guidelines-and-folder-structure-instructions/`.
2. Total file count in `02-spec/21-app/` (excluding the active task) is exactly 2.
