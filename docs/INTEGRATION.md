# Integrating another app

Calendar exposes its shared service client as the public Go package:

```go
import "github.com/oneworkspacestudio/calendar/shared"
```

1. Resolve the selected installation root (`WORKSPACE_SHARED_DATA`, a managed registry discovery result, or an explicit user choice).
2. Register the dependent app with `shared.Install`. Pass initial settings only for the first app; subsequent installations pass `nil`. Conflicting identity/domain/version settings are rejected.
3. Use `shared.Ensure` to find a healthy manager or launch WorkspaceServices.exe. Never spawn stalwart.exe from an app.
4. Perform MyAccounts OIDC code/PKCE login with the installation's dedicated shared client. Register exact callbacks; do not disable audience checks, infer ownership from email, or map IdP groups to backend administrators.
5. Use the user's access token with CalDAV (`/dav/cal/`), CardDAV (`/dav/card/`) or JMAP (`/jmap/session`, `/jmap`). Let Stalwart enforce its account and resource ACLs. Never proxy these operations with the recovery credential.
6. On uninstall call `shared.Uninstall`. It only unregisters the app, and does not stop the manager or delete settings, calendars, contacts, attachments, mail or engine files.

Calendar never depends on Mail. The shared backend supports the protocols even when their companion GUI app is not installed. Discovery of an external Stalwart process or Windows service is deliberately separate from discovery of a managed installation. There is no automatic external adoption operation in this preview.

One Workspace's shared integration adds verified `stalwart-shared` and `services-shared` packages to its catalog. Their presence explicitly opts the new bundle into the shared mode; old published catalogs retain their old runtime. A common audience/client serves the three exact suite callbacks and the manager starts before the first dependent app. The new Mail build accepts `--shared-data` and delegates all native ownership to Workspace Services.

To extend beyond these three applications, add a declared consumer ID to the manager and its callback to the dedicated client, then use the same registration/lifecycle contract and protocol-level per-user tokens. This is an explicit extension boundary, not a promise that arbitrary unregistered native binaries are trusted plugins.
