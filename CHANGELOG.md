# Changelog

Fork of [ngoduykhanh/wireguard-ui](https://github.com/ngoduykhanh/wireguard-ui) `v0.6.2`, which is no longer maintained.

## v0.7.0 - 2026-10-09
### What's Changed
#### 🚀 Features
* Shared client AllowedIPs: one list appended to every client config, with a per-client opt-out. Static entries are edited in Global Settings; with `WGUI_DO_TOKEN` set, the list is refreshed from the public IPv4 of all droplets and all reserved IPs. The endpoint address is never synced into the list.
* New UI theme with light and dark mode; no external font or icon CDN requests.
#### 🔧 CI
* Build and publish `ghcr.io/obervinov/wireguard-ui` through the `obervinov/_templates@v4.0.0` reusable workflows, replacing the upstream Docker Hub, lint and binary release workflows.
