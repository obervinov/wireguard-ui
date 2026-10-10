# Changelog

Fork of [ngoduykhanh/wireguard-ui](https://github.com/ngoduykhanh/wireguard-ui) `v0.6.2`, which is no longer maintained.

## v0.7.1 - 2026-10-10
### What's Changed
#### 🚀 Features
* Client cards list the shared AllowedIPs next to the client's own (dashed badges), so a card shows everything that ends up in the client config.
* The per-client toggle is now **Use shared Allowed IPs** (on by default) instead of **Exclude shared Allowed IPs**; stored data and the API field are unchanged.

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
