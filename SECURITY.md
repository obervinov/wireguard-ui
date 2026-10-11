# Security Policy

## Supported Versions

Only the latest release of this fork gets fixes.

| Version | Supported          |
| ------- | ------------------ |
| 0.8.x   | :white_check_mark: |
| < 0.8   | :x:                |

Upstream [ngoduykhanh/wireguard-ui](https://github.com/ngoduykhanh/wireguard-ui) is no
longer maintained, and this fork is the only place where its code is patched. Report
here even when the issue is in code inherited from upstream `v0.6.2`.

## Reporting a Vulnerability

Report privately through a security advisory:
https://github.com/obervinov/wireguard-ui/security/advisories/new

Please do not open a public issue for it. Include the version, how to reproduce it and
what an attacker gets.

## Trust boundaries

What an attacker gets at each compromise point:

| Compromise point | What it yields |
| --- | --- |
| An admin account in the UI | The whole VPN: every client config with its private key, the server key, the shared AllowedIPs, and the ability to add a client for themselves |
| A non-admin account | Every client config with its private key, and adding, editing and applying clients; users, server and global settings stay admin-only |
| The `db/` directory or a backup of it | Server and client private keys in plain JSON, bcrypt password hashes, TOTP secrets |
| The container | The above, `NET_ADMIN` on the host network namespace, and every `WGUI_*` secret in its environment (`SESSION_SECRET`, `WGUI_DO_TOKEN`, SMTP and Telegram credentials) |
| `SESSION_SECRET` | Forged session cookies for any user, without the password or the second factor |
| A one-time Enclosed link | One client config, once, until it expires (`WGUI_ENCLOSED_TTL`) |

## Recommendations

- **Do not expose the UI to the internet.** Reach it through the tunnel itself or a
  reverse proxy with its own authentication, and terminate TLS there: wgui serves plain
  HTTP and does not set the `Secure` flag on its session cookie.
- **Set `SESSION_SECRET`** to a long random value. Without it a new key is generated at
  every start, which signs everyone out on restart.
- **Turn on 2FA** for every account on the Profile page.
- **Give `WGUI_DO_TOKEN` read scopes only** (`droplet:read`, `reserved_ip:read`): wgui
  never writes to DigitalOcean.
- **Keep `db/` with mode 0700** and treat its backups like the WireGuard keys they hold.

## What it does enforce

- Passwords are stored as bcrypt hashes (cost 14); the user API never returns them.
- TOTP codes are accepted once; 5 wrong codes lock the code step for 5 minutes;
  recovery codes are stored as SHA-256 hashes and work once each.
- Session cookies are `HttpOnly` and `SameSite=Lax`.
- Files under `db/` are written with mode 0600.
- Enclosed notes are encrypted inside wgui; the key travels only in the URL fragment,
  so the Enclosed instance stores ciphertext and never sees the config.
- Login follows `?next=` only to a path on the same host.
