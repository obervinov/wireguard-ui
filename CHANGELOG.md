# Changelog

Fork of [ngoduykhanh/wireguard-ui](https://github.com/ngoduykhanh/wireguard-ui) `v0.6.2`, which is no longer maintained.

## v0.8.2 - 2026-10-10
### What's Changed
#### 📦 Dependencies
* Go 1.21 → 1.26 (`go.mod`); the image builds with `golang:1.27-alpine3.24` and runs on `alpine:3.24` (was 3.19).
* All Go modules updated, among them `golang.org/x/crypto` 0.17 → 0.58, `labstack/echo/v4` 4.11 → 4.16, `labstack/echo-contrib` 0.15 → 0.50, `gorilla/sessions` 1.2 → 1.4, `NicoNex/echotron/v3` 3.27 → 3.46, `wgctrl` 2021 → 2024.
* AdminLTE 3.0.4 → 3.2.0, which brings jQuery 3.7.1 and Bootstrap 4.6.2 (was jQuery 3.4/3.5, Bootstrap 4.4).
* Dependabot checks Go modules, npm, the Dockerfile base images and the workflow templates weekly; minor and patch Go bumps arrive as one PR, AdminLTE 4 is skipped.
#### 🐛 Bug Fixes
* Telegram replies use `ReplyParameters`, the field echotron now takes instead of `ReplyToMessageID`.

## v0.8.1 - 2026-10-10
### What's Changed
#### 📚 Documentation
* README: fork notice (upstream `v0.6.2`), what the fork adds, fresh screenshots in `docs/screenshots/`, GHCR image instead of Docker Hub, thanks to the upstream author with a text link to the author's Buy Me a Coffee page. The repository no longer shows a **Sponsor** button (`.github/FUNDING.yml` pointed at the upstream author). The docker-compose examples use `ghcr.io/obervinov/wireguard-ui`.
#### 🐛 Bug Fixes
* Global Settings: the Shared Allowed IPs help still referred to the old **Exclude shared Allowed IPs** toggle; it now names **Use shared Allowed IPs**.

## v0.8.0 - 2026-10-10
### What's Changed
#### 🚀 Features
* One-time share links for client configs through an [Enclosed](https://github.com/CorentinTh/enclosed) instance (`WGUI_ENCLOSED_URL`, `WGUI_ENCLOSED_TTL`): **Share** on a client card encrypts the config in wgui and returns a link plus its QR; the note holds the config as text and as a `.conf` attachment, is deleted after the first read, and the key never reaches the instance.
* Two-factor authentication: TOTP (any authenticator app) enabled per user on the Profile page, with 8 one-time recovery codes. Sign-in asks for the code after the password; 5 wrong codes lock the code step for 5 minutes, and a code is accepted once. Admins can reset another user's 2FA on the Users page.
* Client cards list the shared AllowedIPs next to the client's own (dashed badges), so a card shows everything that ends up in the client config.
* The per-client toggle is now **Use shared Allowed IPs** (on by default) instead of **Exclude shared Allowed IPs**; stored data and the API field are unchanged.
#### 🔒 Security
* `GET /get-users` and `GET /api/user/:username` no longer return password hashes; they return the username, role and 2FA state only.
#### ⚠️ Upgrade notes
* The user record gained fields, so every existing session ends once after the upgrade: sign in again.

## v0.7.0 - 2026-10-09
### What's Changed
#### 🚀 Features
* Shared client AllowedIPs: one list appended to every client config, with a per-client opt-out. Static entries are edited in Global Settings; with `WGUI_DO_TOKEN` set, the list is refreshed from the public IPv4 of all droplets and all reserved IPs. The endpoint address is never synced into the list.
* New UI theme with light and dark mode; no external font or icon CDN requests.
#### 🐛 Bug Fixes
* `POST /login` without `rememberMe` (or without `username` / `password`) crashed the handler with an interface conversion panic and returned an empty reply; the fields are now read with checked assertions and a missing one is rejected.
#### 🔧 CI
* `init.sh`: run the given command instead of the app when arguments are passed, so `docker run <image> uname -m` in the template validation exits instead of starting the server.
* Build and publish `ghcr.io/obervinov/wireguard-ui` through the `obervinov/_templates@v4.0.0` reusable workflows, replacing the upstream Docker Hub, lint and binary release workflows.
