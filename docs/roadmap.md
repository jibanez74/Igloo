# Roadmap and Known Follow-up Work

Igloo is pre-v1 software. This document tracks planned work and known follow-up items that are too detailed for the README.

## Near-term Priorities

- Add scan-status endpoints or events so the web client can distinguish "scan started" from "scan completed."
- Improve watch-room WebSocket fan-out so slow clients cannot delay events for everyone else in a room.
- Finish library management controls for adding, removing, and rescanning configured media paths from the web UI.
- Continue hardening direct-stream and HLS playback behavior across browsers, codecs, subtitles, and network conditions.
- Keep `docs/openapi.json` aligned with the server API as routes evolve.

## Planned Feature Areas

- TV shows: scanning, metadata, seasons, episodes, playback, and watch progress.
- Photos: likely integration with Immich instead of rebuilding a full photo platform inside Igloo.
- Dedicated clients: TV and mobile clients that use the same server API as the web client.
- More complete admin and maintenance workflows for self-hosted deployments.

## Known Follow-up Items

- `server/cmd/api/watch_room_ws.go`: broadcast delivery should be resilient to slow clients. The current server-side path writes to sockets serially; a future version should use per-client buffered outbound queues with a single writer loop per client and non-blocking room broadcast fan-out.
- `web/src/routes/_auth/settings/libraries.lazy.tsx`: movie scan cache invalidation happens immediately after `triggerMovieScan()` returns, before the background scan is complete. Add scan status polling or completion events, then invalidate movie stats, library, latest movies, and playlist queries when the scan actually completes.
- `web/src/routes/_auth/settings/libraries.lazy.tsx`: the movie stats section currently treats backend failures like an empty library and can render `0 Movies` on error. Add a distinct unavailable/error state so failures are visible.
