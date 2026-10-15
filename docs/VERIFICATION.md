# Verification

## Accepted locally

| Check | Evidence |
| --- | --- |
| Every app installation order | All six permutations against the real pinned Stalwart executable |
| One backend | Every later registration retains the same native process ID and engine path |
| Calendar without Mail | Calendar-first native tests and Calendar's standalone EXE |
| Default network surface | Exactly one private HTTP listener; no mail listener enabled by app installation |
| MyAccounts-compatible OIDC | Synthetic issuer with signed tokens, discovery/JWKS, nonce and PKCE; native bearer authentication and full application callbacks |
| User isolation | Separate subjects; native calendar/contacts/mail access controls and API checks |
| Calendar | Native CalDAV discovery/create/read, ETag conflict rejection, CSRF rejection |
| Contacts | Native CardDAV create/read and app sign-in/read |
| Mail | The shared-mode standalone Mail.exe completes OIDC and uses JMAP; closing it leaves Stalwart running |
| Uninstall | All six removal sequences preserve the shared service and stored calendar/contact data, including after the final app is removed |
| Recovery | Native process termination in every installation order followed by managed recovery and retained data |
| Maintenance | Explicit manager stop, offline backup, pinned-package reapplication and planned restart preserve stored events; live/unknown-version upgrades rejected |
| Calendar logic | DST conversion, weekly local-time recurrence across DST, a single edited occurrence, all-day exclusive end, invalid interval rejection preservation of UID/custom properties/URLs/guests/reminders, late-night intervals, inherited visibility and stable draft UIDs on retry |
| Packaging | Locally compiled Windows app/manager EXEs, separate verified official Stalwart binary, production web builds, notices/source/checksums |

UI checks use synthetic events only. Browser review covers the week view and event create/save flow; additional views and responsive captures are provided in the screenshots folder. The interface follows the inspected Google Calendar desktop layout, with original local styling and One Workspace branding.

## Boundaries

- Native auth acceptance uses a synthetic OIDC issuer with the MyAccounts-compatible flow; it is not a claim of a public production MyAccounts/Cloudflare deployment.
- No existing live Workspace/Mail/identity installation was used for tests or upgraded.
- No external mail, public SMTP, DNS, router or Cloudflare change was made for this task.
- This is not a clean Windows host, SCM service, logon/reboot, cross-machine restore, or future Stalwart schema-migration acceptance.
- The shared manager runs in the installing user's Windows session. Private state requires NTFS; payloads may use other filesystems.
- Google-specific services, public booking pages, a full sharing-management UI and invitation delivery are not asserted as implemented.

Run the native checks using the opt-in environment variables described in the technical guide. Never point test variables at a real data directory.
