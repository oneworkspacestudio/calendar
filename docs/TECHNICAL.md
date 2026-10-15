# Technical guide

## Windows installation

The portable package contains Calendar.exe, Contacts.exe, the shared manager WorkspaceServices.exe, the **separate unmodified official Stalwart executable**, engine.lock.json, notices and checksums. The optional Mail.exe is built from the shared-backend Mail integration source; Mail does not embed Stalwart in this mode.

Run `Calendar.exe`, `Contacts.exe`, or the selected app through:

```powershell
./scripts/install-app.ps1 -App calendar -Programs I:/OneWorkspace/Apps -SharedData C:/WorkspacePrivate/Shared
```

Use the same `-SharedData` for every app belonging to an installation. Program payloads can use exFAT; private state requires NTFS for ACL protection. Existing registered managed installations can be discovered automatically when there is exactly one readable match. Use `--shared-data PATH` when several installations exist.

The first application's setup screen asks for the MyAccounts issuer URL, shared client ID and client secret. The official engine path and checksum are supplied by the package. These are installation-owner settings, not information each ordinary user enters. Each user subsequently signs in with MyAccounts.

Use **one dedicated shared MyAccounts client** for Calendar, Mail and Contacts, with exact callbacks, for example:

- `http://127.0.0.1:8998/auth/callback` (Calendar)
- `http://127.0.0.1:8999/auth/callback` (Contacts)
- `http://localhost:19496/auth/callback` (standalone Mail)

For suite hosting, the installer registers one shared client with `/calendar/auth/callback`, `/contacts/auth/callback` and `/mail/auth/callback` on the suite HTTPS origin. The Stalwart OIDC directory enforces that shared audience. Issuer and immutable `sub` determine account ownership; contact email does not grant access or administrator privileges. A single audience avoids disabling native audience validation to accommodate multiple apps.

Calendar and Contacts flags: `--shared-data`, `--data`, `--listen`, `--origin`, `--base`, `--services`, `--no-browser`, `--version`. The listener is loopback only. Use the authenticated One Workspace proxy or a correctly configured HTTPS reverse proxy for public access. Configuring Calendar never opens SMTP or changes DNS/Cloudflare.

`--demo` is an explicit synthetic local demonstration. It does not connect MyAccounts or a real backend. The UI labels this mode, and generated screenshots use synthetic events only.

## Shared service

Source: `shared/`; executable: `cmd/services`.

The background manager owns one Stalwart child in a Windows Job. Apps discover its protected installation registry/state and health endpoint. A named Windows mutex plus a deny-sharing runtime file lock prevent duplicate ownership/startup. A separate install lock serialises registration and maintenance. HKCU installation records identify only managed installations; external process/Windows-service discovery is informational and never adopts or modifies those instances.

The manager remains independent of individual app lifetimes and registrations, including after the last app is removed. It runs in the installing Windows user's session. Optional HKCU logon startup is explicit; this preview does not claim an SCM service/boot-without-logon installation or reboot acceptance.

```powershell
./WorkspaceServices.exe discover
./WorkspaceServices.exe status --root C:/WorkspacePrivate/Shared
./WorkspaceServices.exe run --root C:/WorkspacePrivate/Shared
./WorkspaceServices.exe stop --root C:/WorkspacePrivate/Shared
./WorkspaceServices.exe external
./WorkspaceServices.exe autostart --root C:/WorkspacePrivate/Shared --enabled
```

The default native listener is **private HTTP only**, serving JMAP, CalDAV and CardDAV. Installing Mail does not activate SMTP, IMAP or POP3. Native crash/unresponsive checks trigger bounded-backoff restart without app-owned backend processes. User requests retain each user's own token; the manager's protected recovery credential is used only for native configuration and health checks.

Native Stalwart recovery/bootstrap temporarily binds its password-protected recovery HTTP listener on all interfaces on the selected high port, as implemented by the official engine. It never opens public SMTP during bootstrap. Normal operation replaces this with the managed loopback listener before exposing apps. Treat initial setup as owner maintenance on a trusted host/network; this is a documented upstream behaviour, not a claim of a loopback-only bootstrap.

## Persistent layout

```text
Shared/
  ownership.json       # scoped managed ownership; nonempty external folders are refused
  settings.dpapi       # authoritative installation identity/engine settings
  service.json         # schema, registered consumers and engine version/hash
  engine.json          # Stalwart's persistent registry configuration
  recovery.dpapi       # configuration/health credential
  control.dpapi        # rotating local manager control credential
  runtime.lock         # exclusive service ownership
  install.lock         # serialised registration/maintenance
  running.json         # instance identity, health, owned PIDs and recovery count
  bin/                 # hash-verified engine versions
  store/               # Stalwart database, blobs, calendars and address books
  apps/                # independent per-app preferences and setup links
  backups/             # stopped maintenance snapshots
```

Stalwart is the data source of truth. App preferences are keyed by an exact issuer/subject hash. ACLs and DPAPI protect local settings; browser tokens remain server-side in HTTP-only sessions. OIDC uses state, nonce and PKCE. Resource writes require CSRF and ETag conditions; stale edits return a conflict instead of overwriting newer data. DAV input cannot select an arbitrary proxy host or a non-DAV endpoint.

## Backup and recovery

Stop the shared service for a consistent filesystem copy. Apps must be closed for coordinated maintenance, but uninstalling an app never stops the service.

```powershell
./WorkspaceServices.exe stop --root C:/WorkspacePrivate/Shared
./WorkspaceServices.exe backup --root C:/WorkspacePrivate/Shared --output E:/Backups/Workspace-2026-10-15
./WorkspaceServices.exe run --root C:/WorkspacePrivate/Shared
```

The backup command requires a new protected NTFS destination; choose another NTFS disk or create a protected NTFS backup before exporting an encrypted archive to other media. It copies persistent state without deleting the original. DPAPI files are machine-bound: moving a backup to another Windows machine requires explicit secret re-enrolment and a migration procedure. Whole-system backup is preferable if you need to retain machine protection keys. Cross-machine restore has not been accepted in this preview.

`upgrade --settings NEXT.json --root PATH` is an explicit stopped maintenance operation. It refuses identity/domain/listener changes, verifies the proposed binary, saves stopped store/settings snapshots, and retains prior engine files. Only the pinned tested engine version is currently compatible. Reapplication/restart and rejection of unknown versions are tested; forward-schema migration to a future Stalwart release is not asserted. Never replace a live engine or reset its database to make an upgrade pass.

## Optional mail listeners

Only the owner can run explicit offline listener maintenance. This changes only the manager's named mail listeners and retains other native settings.

```powershell
./WorkspaceServices.exe stop --root C:/WorkspacePrivate/Shared
./WorkspaceServices.exe mail-listeners --root C:/WorkspacePrivate/Shared --smtp-port 2525 --imap-port 2143
./WorkspaceServices.exe run --root C:/WorkspacePrivate/Shared
```

Public SMTP additionally requires both `--public-smtp-port 25` and **`--allow-public-smtp`**. The public listener enables STARTTLS support; valid certificate and real Internet mail configuration remain administrator tasks. DNS, MX, SPF/DKIM/DMARC, routing/firewall and certificate setup are not performed by Calendar or this command. HTTP Cloudflare tunnels do not carry SMTP. No public listener, DNS record, router setting or external message was changed/sent during acceptance testing.

Calling `mail-listeners` with a zero/omitted port disables that manager-owned mail listener. App installation never invokes this command. Existing legacy Mail storage must be migrated explicitly; shared mode refuses silent adoption and does not give Mail the shared recovery credential.

## Build locally

Windows x64, Go 1.27 and Node 26 were used. No GitHub Actions. Sources and dependency versions are locked.

```powershell
npm ci --prefix web
./scripts/build.ps1 -Go go
```

The build script verifies the official Stalwart 0.16.26 ZIP SHA256 before extracting the separate executable. Its exact source is pinned to commit `18f4e9505027d790fd7e9fdad7f922be6719101d`. No local Rust rebuild is claimed.

UI is authored in `web/src/Calendar.aui` and compiled with `@codedia/parser` to tracked generated TSX. Native element adapters live in `web/src/components/ui.tsx`; date, recurrence and timezone logic is separate in `calendar.ts`. Production web assets are embedded into each app executable. Google application CSS and private reference events are not distributed.

```powershell
go test ./...
node --test web/test/calendar.test.mjs
$env:WORKSPACE_TEST_STALWART=(Resolve-Path dist/stalwart.exe).Path
$env:WORKSPACE_TEST_MAIL=(Resolve-Path dist/Mail.exe).Path
go test ./... -v -count=1
```

Native tests create synthetic OIDC identities, fresh NTFS stores and loopback listeners. They must never target an existing installation. The normal test suite skips native-engine cases unless explicitly opted in.
