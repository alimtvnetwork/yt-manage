# Add IPs to Muse Prompt in Coding Guideline: high priority instruction, non-negotiable task

## slug: add-ips-to-muse-prompt-in-coding-guideline

## 1. Request (verbatim intent)
Add every IP address Muse has requested (network-access requests the operator approved for Muse sessions) to the Muse master prompt in coding-guidelines-v24, update the companion skill, then commit and push.

## 2. Source of the IP list
The authoritative record is the environment's granted network permissions (durable "allow" rules created when Muse requested access and the operator approved). Extracted 2026-10-23. The list contains:

- 41 IP endpoints, all `198.19.0.x`, all on port 22 (SSH): 198.19.0.2, 198.19.0.3, 198.19.0.4, 198.19.0.5, 198.19.0.6, 198.19.0.7, 198.19.0.52, 198.19.0.54, 198.19.0.73, 198.19.0.110, 198.19.0.113, 198.19.0.114, 198.19.0.116, 198.19.0.120, 198.19.0.125, 198.19.0.126, 198.19.0.131, 198.19.0.134, 198.19.0.137, 198.19.0.150, 198.19.0.154, 198.19.0.155, 198.19.0.156, 198.19.0.157, 198.19.0.158, 198.19.0.166, 198.19.0.167, 198.19.0.168, 198.19.0.171, 198.19.0.174, 198.19.0.176, 198.19.0.177, 198.19.0.196, 198.19.0.204, 198.19.0.210, 198.19.0.212, 198.19.0.238, 198.19.0.243, 198.19.0.247, 198.19.0.249, 198.19.0.252.
- Hostname grants (not IPs, recorded in the same section for completeness): `github.com` (ports 22, 80, 443, including `**.github.com`), `nodesource.com` (ports 80, 443, including `**.nodesource.com`).

Excluded as test artifacts, disclosed here rather than silently dropped: `invalid-target` (port 22), `b` (port 22), `remote.lan` (port 22), `192.0.2.1` (ports 2222 and 54321 — TEST-NET-1 documentation range). If the operator wants them recorded too, they can be appended in a follow-up.

## 3. Changes
1. `01-prompts/27-muse-prompts/01-muse-master-prompt.md`: add a new section "Requested Network Endpoints (IPs Muse Has Requested)" after the Master Verification Checklist and before the version footer; bump Prompt Version 6.5.0 -> 6.5.1 (header + footer).
2. `01-prompts/27-muse-prompts/readme.md`: update the catalog row for `01-muse-master-prompt.md` to 6.5.1 and mention the endpoint registry in its scope summary.
3. `.agents/skills/muse-master-prompt/skill.md` and `.cursor/skills/muse-master-prompt/skill.md`: mirror the same endpoint section (the "update the skill" half of the request).
4. `02-muse-generic.md` is intentionally NOT changed: it is the public generic prompt with zero hardcoded environment specifics by design.

## 4. Invariants
- Repository-relative paths only, everywhere including this spec.
- Search via GitMap only; pull before work; one atomic commit + push via GitMap.
- The IP list is copied verbatim from the permission record; no address may be added, removed, or renumbered by inference.
