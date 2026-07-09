# Silo Requests: Seerr

A Silo `request_router.v1` plugin that fulfills content requests by submitting
them to a [Seerr](https://github.com/seerr-team/seerr) (Overseerr/Jellyseerr-
compatible) instance. Seerr manages its own Sonarr/Radarr; this plugin is a thin
adapter.

## Connection config

Each connection carries a Seerr **base URL** + **API key** (host chrome) and one
plugin setting:

- **This Seerr handles 4K requests** (`supports_4k`, default off) — enable only
  if the Seerr instance has a 4K Sonarr/Radarr configured. When off, 2160p
  requests are not sent to this connection.

## API key requirement

The API key (Settings → General in Seerr) **must belong to a Seerr admin /
auto-approve user**. Silo is the sole approval authority; requests created via
the API auto-approve and hand off to Seerr's Sonarr/Radarr immediately. A
non-admin key would leave requests pending in Seerr (visible as `queued` in Silo
that never advances). `TestConnection` (`GET /api/v1/auth/me`) surfaces an
invalid key.

## How requests map

- Each requested quality becomes one Seerr request: HD → `is4k:false`, 2160p →
  `is4k:true` (skipped when `supports_4k` is off).
- `series` → Seerr `mediaType: "tv"` with `seasons: "all"`; movies use
  `mediaType: "movie"`. Media is identified by **TMDB id**.
- A duplicate (HTTP 409) is treated as already-queued; the plugin recovers the
  existing Seerr request id so Silo can track it.

## Build / test

```
go build ./... && go test ./...
```

## Community maintenance

This is an approved community plugin maintained in the
[`Silo-Community`](https://github.com/Silo-Community) organization. Use
[GitHub Issues](https://github.com/Silo-Community/silo-plugins-requests-seerr/issues)
for support and bug reports. Security reports should follow
[`SECURITY.md`](SECURITY.md).

The plugin consumes the published
[`silo-plugin-sdk`](https://github.com/Silo-Server/silo-plugin-sdk); CI rejects
machine-local SDK replacement directives.

## License

Licensed under AGPL-3.0. See [`LICENSE`](LICENSE).
