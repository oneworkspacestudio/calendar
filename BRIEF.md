# One Workspace — My Calendar and Shared Stalwart Service

## Product request

Create a new app in the oneworkspacestudio GitHub organisation, a Google Calendar 1:1 clone with an identical UI. Use the current desktop UI as the visual reference, with One Workspace application branding.

## Shared service brief

Implement Stalwart as a managed shared dependency for My Mail, My Calendar and My Contacts.

- Each application must remain independently installable and launchable.
- Only one managed Stalwart instance may run per One Workspace installation.
- Build a central service manager with installation, discovery, health checks, version management and recovery.
- My Calendar must function without My Mail being installed.
- Installing another dependent app must reuse the same backend without resetting existing data.
- Configure MyAccounts OIDC integration with secure account mapping and per-user access controls.
- Use JMAP or CalDAV for Calendar, JMAP or IMAP for Mail, and JMAP or CardDAV for Contacts.
- Package Windows applications as standalone executables. Keep Stalwart as one separate shared executable.
- Store settings, databases, attachments and calendars in persistent, backup-friendly folders.
- Use one authoritative Stalwart configuration and prevent duplicate startup through a service-level lock and registry checks.
- Never silently delete the shared backend or user data when an application is uninstalled.
- Enable public SMTP and mail-specific listeners only when explicitly configured.
- Detect externally installed Stalwart instances but never modify or adopt them without administrator consent.
- Add automated installation tests for every app installation order, upgrade, restart and uninstall scenario.

Acceptance: Installing My Calendar, then My Mail, then My Contacts results in exactly one Stalwart service, with all three applications working through MyAccounts. Removing any individual application must not interrupt the others.

## Delivery boundaries

The implementation and verification guide records accepted flows, visual-reference coverage and remaining Google-specific capabilities. Do not treat a preview, a synthetic OIDC issuer or one development machine as full Google feature parity, production MyAccounts/HTTPS, clean-host or reboot acceptance.
