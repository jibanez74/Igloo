# OpenAPI maintenance

`docs/openapi.json` is the source of truth for Igloo's HTTP API documentation.

When adding or changing an API route:

1. Update the route or handler in `server/cmd/api`.
2. Update the matching path, parameters, request body, responses, and reusable schemas in `docs/openapi.json`.
3. Run the OpenAPI route coverage test from `server/`:

```sh
go test ./cmd/api -run TestOpenAPIDocumentsRegisteredAPIRoutes
```

Conventions:

- JSON responses use the shared `{ "error": boolean, "message"?: string, "data"?: object }` envelope.
- Session-protected routes use `cookieAuth`; public routes set `security: []`.
- Admin-only routes still use `cookieAuth`, include `403`, and state the admin requirement in the description.
- Streaming routes document `206` when range requests are supported.
- HLS playlists, HLS segments, WebVTT subtitles, static assets, and WebSocket upgrades are documented even though they do not use the JSON envelope on success.
