# Code Signing Policy — ClickRAX

## Signed artifacts

- `clickrax.exe` — GUI application
- `clickrax-cli.exe` — command-line tool
- NSIS installer (when published)

## Release process

1. Tag `v*` on the main branch.
2. GitHub Actions builds binaries from source (no manual edits to release artifacts).
3. SignPath Foundation signs artifacts after maintainer approval.

## Team roles

| Role | Responsibility |
|------|----------------|
| Committers | Push to repository; MFA required on GitHub |
| Reviewers | Review pull requests before merge |
| Approvers | Approve signing requests in SignPath.io |

## Privacy

ClickRAX does not transfer data to third-party servers except:

- Proxmox Backup Server, SMB, or FTP destinations configured by the user
- Optional webhook URL configured by the user
- Optional SMTP server configured by the user

See [privacy.md](privacy.md).

## Reporting security issues

Report vulnerabilities privately — see [SECURITY.md](../SECURITY.md). Do not use public Issues for sensitive reports.
