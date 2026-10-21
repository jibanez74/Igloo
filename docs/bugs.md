# OpenAPI audit findings

Recorded on 2026-10-21 during the audit of [openapi.json](openapi.json).

The three production issues below were reproduced using isolated test databases
and temporary test files. Production code was left unchanged at the user's
request. The reproduction tests were temporary and are not committed; add
permanent regression coverage when implementing fixes. Suggested fixes are
starting points for a later planning session, not approved implementation plans.

## Open: missing general settings fields silently clear configuration

**Endpoint:** `PUT /api/settings/general` (administrator authentication required).

**Source:** `UpdateGeneralSettings` and `updateGeneralSettingsRequest` in
[settings_handler.go](../server/cmd/api/settings_handler.go).

The OpenAPI request schema requires all 11 settings fields because this endpoint
replaces the complete configuration. The handler decodes into ordinary strings
and booleans, so omitted fields become empty strings or `false`. It validates
that the two directory fields are nonempty but does not check whether the other
required fields were provided before saving all fields.

### Reproduction and observed result

1. In an isolated test application, save general settings containing a TMDB key,
   a Spotify client secret, and enabled boolean flags.
2. Authenticate as an administrator and send a request containing only
   `static_dir` and `transcode_dir`, both pointing to accessible temporary
   directories.
3. Observe HTTP `200`. The saved credentials are cleared and omitted boolean
   flags become `false`.

**Expected:** Reject a request missing any required field with HTTP `400` and
leave the saved settings unchanged. Explicit empty strings and `false` remain
valid values where the contract permits them.

**Impact:** An incomplete client request can silently erase configuration. The
specification's required fields were retained; weakening them would conceal the
implementation mismatch.

**Suggested fix and regression coverage:** Track field presence during decoding
and validate the complete request before filesystem or database changes. Test
each omitted required field, explicit empty/false values, and preservation of
existing settings after rejected requests. Review explicit JSON `null` handling
against the non-nullable schema as part of this work.

## Open: concurrent Quick Connect redemption issues multiple device tokens

**Endpoint:** `POST /api/quick-connect/redeem`.

**Sources:** `RedeemQuickConnect` in
[quick_connect_handler.go](../server/cmd/api/quick_connect_handler.go), and
`QuickConnectBroker.Redeem` / `Consume` in
[quick_connect.go](../server/cmd/api/quick_connect.go).

The broker checks approval under a mutex, but releases the mutex before the
handler creates a device token. The handler consumes the code only after token
creation succeeds. Two requests can therefore both observe the same approved
entry and create tokens before either consumes it. The existing `Consume`
comment acknowledges this concurrency window.

### Reproduction and observed result

1. Initiate and approve one Quick Connect code in an isolated test application.
2. Limit the test database to one connection and temporarily hold that connection.
3. Start two concurrent redemption requests with the same code and device-held
   secret. Wait until both are blocked waiting for a database connection, after
   they have passed the broker's approval check.
4. Release the database connection and allow both requests to finish.
5. Observe two HTTP `200` approved responses containing distinct tokens, with two
   device rows created from the single approval.

**Expected:** A single approved code can successfully issue only one device
token, including under concurrent requests. A failed issuance should still allow
an appropriate retry.

**Impact:** One approval can create multiple valid device credentials. Revoking
one device does not revoke the other. Both requests need the device-held secret;
this reproduction does not demonstrate an authentication bypass.

**Suggested fix and regression coverage:** Atomically claim an approved entry
before issuance, consume it on success, and release the claim on issuance
failure. Define the response for another request arriving during issuance. Add a
deterministic concurrent handler test asserting one token/device, plus coverage
for retry after issuance failure and redemption after consumption.

## Open: track pagination overflows for a very large offset

**Endpoint:** `GET /api/music/tracks` (authentication required).

**Source:** `GetTracksAlphabetical` in
[track_handler.go](../server/cmd/api/track_handler.go).

The query parser accepts the maximum signed 64-bit offset. The handler computes
`has_more` using `offset + limit < total`; adding the default limit of 50 to that
offset overflows to a negative value.

### Reproduction and observed result

1. Use an isolated test application with an empty music library.
2. Send an authenticated request to
   `/api/music/tracks?offset=9223372036854775807`.
3. Observe HTTP `200`, `total: 0`, no tracks, and `has_more: true`.

**Expected:** An empty library, or an offset beyond the last track, must not
report more results. If the accepted query range changes instead, document and
test the chosen rejection behavior.

**Impact:** Pagination metadata is incorrect for extreme inputs and can mislead
clients into requesting further pages.

**Suggested fix and regression coverage:** Use an overflow-safe comparison,
guarding the offset before subtracting from the total. Test empty libraries,
offsets at and beyond the total, ordinary page boundaries, and the maximum
accepted offset. Review other pagination arithmetic for the same pattern; only
this track endpoint was confirmed affected during this audit.

## Documentation findings already corrected

These changes were applied during the audit and are not outstanding production
fixes:

- Account name/email limits apply after trimming Unicode whitespace. Removed
  inaccurate raw-input maximum lengths and documented the trimmed limits and
  lack of email-format validation. Added missing nonempty constraints for login
  credentials, current passwords, and relevant email fields.
- HLS authentication/session middleware can return `401` or `500` before the
  handler sets `Cache-Control: no-store`. Corrected the endpoint descriptions
  and [ffmpeg.md](ffmpeg.md), retaining the existing plain-text `416` exception.
- Later TMDB enrichment can overwrite manual movie edits. Identifying a movie
  pins its identity and clears retries but does not protect edits from future
  enrichment; corrected the movie update description.
- An administrator resetting their own password preserves the requesting browser
  session only with cookie authentication. A device token used for the reset is
  revoked along with the account's other device tokens; clarified the description.
- The cookie-only route list in
  [openapi-maintenance.md](openapi-maintenance.md) omitted email changes, password
  changes, and account deletion; added those routes.
- Replaced redundant single-branch `allOf` wrappers for `HealthResponse`,
  `AdminUser`, and `ReorderTracksRequest` with direct references, eliminating the
  three OpenAPI lint warnings.
- Regenerated frontend OpenAPI types and added contract tests for trimmed field
  boundaries and HLS middleware error headers.

## Validation at the end of the audit

`make check` passed, including OpenAPI lint and route coverage, generated-type
drift verification, server lint/tests, web lint, 1,537 web tests, type checking,
and the production web build. The generated-type check used a temporary Git
index containing the newly generated file so it could verify reproducibility
without staging changes in the user's index. Passing these checks does not
resolve the three open issues above.
