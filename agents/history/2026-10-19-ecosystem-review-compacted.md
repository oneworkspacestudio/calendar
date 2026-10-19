# Ecosystem review — local changes

Contacts title/favicon match its identity. Calendar and Contacts share workspace-theme with other apps and reuse ready Workspace app switcher. Incoming storage changes never write back. Tracked web asset/index changes are intentional.

Current user asked to review/fix ecosystem consistency. Source changes remain
uncommitted/unpublished; no release/version pin changed and no GitHub Actions.
Use the suite history/docs/ECOSYSTEM-REVIEW.md for the complete cross-app audit.
AUI sources regenerated where changed; TS/Vite and native Windows builds passed.
Twelve managed synthetic browser regressions passed in the full19-app fixture;
all19 entries plus Backups were opened. Four-app frontend tests19 checks passed;
Monitor readiness and targeted suite Go regressions passed. This is not exhaustive
feature, Internet mail, Vault unlock, clean-host/reboot or security acceptance.

Theme feedback and Search reconnect new-tab behavior were found during the review
and fixed before final acceptance. Native builds/evidence remain under
I:/Projects/tooling/ecosystem-review. Fresh protected NTFS fixture only; owner data
was never used. Task manager/shared services stopped; named C fixture and browser
state safely removed (779,548,443 bytes); no preview/tunnel remains. See root
one-workspace/agents/history/2026-10-19-ecosystem-review-compacted.md for next steps.
Publication requires authorization for the current task and coordinated suite pins.
## Authorized release follow-up

User now explicitly requested push and release without GitHub Actions. Preparing
`v0.1.0-preview.3` from these reviewed changes with a local Windows build.
The coordinated manager release is One Workspace `v0.3.0-preview.38`. Check the
GitHub release assets and tag before claiming publication complete. Prior review
limitations remain unchanged.

## Published release

Published [v0.1.0-preview.3](https://github.com/oneworkspacestudio/calendar/releases/tag/v0.1.0-preview.3) from `f683add9fd4e8cfdb7ff71a2bfa1cca55726a691`.
All release asset sizes/SHA256 matched the local packages before publication;
the public anonymous Windows ZIP download was separately hashed successfully.
Actions remain disabled. Source and native packages were built locally. Matching
Workspace preview.38 pins include this exact release. Prior product/acceptance
limits above remain; owner installation and data were not changed.
