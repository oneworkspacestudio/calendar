# Calendar and shared Stalwart implementation

User requested a Calendar app in oneworkspacestudio with Google Calendar desktop
visual fidelity, and one shared Stalwart dependency for Calendar/Mail/Contacts.
Original brief is BRIEF.md; technical/integration/verification docs are under docs/.

## Source and delivery

New repo: https://github.com/oneworkspacestudio/calendar (main), I:/Projects/my-calendar.
Calendar.exe and Contacts.exe embed original React/.aui UI. WorkspaceServices.exe
owns one separate official unmodified Stalwart 0.16.26 executable. Exact upstream
source commit18f4e9505027d790fd7e9fdad7f922be6719101d; ZIP source supplied.
No GitHub Actions; confirmed disabled in Calendar, Mail and One Workspace.

SDK package shared/ owns Prepare/Install/Load/Ensure/Run/Uninstall/Stop/Backup/
Upgrade, managed registry/process discovery, explicit optional mail listeners,
Windows mutex/runtime lock/install lock, protected persistent state and recovery.
Nonempty unmanaged/legacy data folders are refused before ACL or file changes.
Only private HTTP starts by default. App uninstall never stops/deletes backend/data.
One shared MyAccounts client/audience has the three exact callback URLs; native
OIDC uses immutable sub and per-user CalDAV/CardDAV/JMAP tokens. Recovery auth is
configuration/health-only. Public SMTP requires an explicit extra CLI consent.

UI covers core calendar views, compact/expanded event editor, calendar toggles,
search, recurrence/exception edits, local reminders, meeting/guest details, ICS
import/export, responsive layout, fullscreen settings and a basic Contacts app.
Timezone definitions from Intl cover1970–2100. Stable draft UIDs prevent retries
creating a second resource. Default visibility inherits rather than forces PUBLIC.
Google Meet/Tasks synchronisation, public booking pages, a full sharing admin UI,
and invitation delivery are not implemented; this is an unsigned preview rather
than a claim of full Google feature parity.

## Verification

- All six native app-registration installation/removal orders, exactly one
  native PID/path, ordinary-user isolation for Calendar/Contacts/Mail, no default
  SMTP/IMAP listeners, persistence after removal (including last app), native
  crash recovery, explicit stop, offline backup, pinned-package reapply/upgrade
  and planned restart passed against real Stalwart with synthetic OIDC.
- Calendar/Contacts OAuth callbacks (signed token, nonce, PKCE), native calendar
  discovery/create, stale ETag and CSRF rejection passed. Shared-mode Mail.exe
  completed OAuth/JMAP; closing Mail left the native backend alive.
- Unit tests cover ownership refusal, duplicate mutex/runtime ownership, registry,
  install order/data retention, maintenance locks/snapshots/version rejection,
  external detection and explicit public-listener consent.
- Eight UI logic tests cover DST, recurring wall times, single exceptions,
  preservation/UIDs, all-day/overnight dates, inherited visibility and retry IDs.
- Browser inspected Google reference; no private reference events/assets saved.
  Synthetic week/month/mobile/contacts captures are in docs/screenshots. Browser
  caught a misplaced AUI Save control; source fixed and create/save/edit verified.
  Compiler now rejects empty structural branches and nested buttons.
- Mail and Workspace unit suites pass. Full fresh suite installer, actual public
  MyAccounts/Cloudflare, clean host, SCM/logon reboot, external mail, cross-machine
  restore and future engine schema migration are not accepted by these checks.

## Related repositories and concurrency

Primary Mail and Workspace checkouts had unrelated concurrent uncommitted network/
installer changes. They were preserved. Isolated worktrees for this task:
I:/Projects/mail-calendar-shared and I:/Projects/workspace-calendar-shared, branches
codex/shared-stalwart in their respective repositories. Only own changes should be
committed/pushed to the oneworkspacestudio remotes, with normal fetch/rebase if needed.
Mail no longer embeds Stalwart in shared source, delegates native lifetime, refuses
legacy auto-adoption and keeps explicit --legacy-engine compatibility. Workspace
opts in via verified stalwart-shared/services-shared catalog packages, registers
one shared client, starts service before dependents, and supplies independent
Calendar/Contacts catalog entries. Calendar does not depend on Mail.

## Finishing state

Publication still pending at this handoff. Planned Calendar module tag/release is
v0.1.0-preview.1; Mail/Workspace go.mod require that SDK tag. Development builds use
an external go.work override until the tag exists. Before final delivery: finish
screenshots/notices, commit/push/tag Calendar, resolve actual module checksums in
Mail/Workspace, commit/push their own integration changes, build exact-source EXEs,
package release/source/licences/checksums, verify uploaded digests and publish.
Do not claim publication until GitHub confirms it. Existing live installations
were not changed. Demo services are task-owned loopback18998/18999 with synthetic
NTFS state under C:/Projects/.cache/calendar-check; build logs outside Git under
H:/Projects. Program/source/artifact storage stays on I:/Projects/my-calendar.
