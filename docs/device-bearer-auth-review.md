# Device bearer authentication review notes

This document expands on two review comments for the device bearer authentication
patch. The patch adds long-lived device tokens with the `igd_` prefix and
resolves them in `DeviceTokenAuth`. That middleware is currently installed near
the top of `InitRouter`, before both WebSocket routes and the normal session
route group.

The goal of the feature is sound: TV and mobile clients need a bearer-token
auth path that does not create cookie sessions. The issues below are about auth
semantics at route boundaries. A revoked or stale device token should not trap a
client out of recovery flows, and a logout response should not claim success
while leaving the bearer token valid.

## Public pairing and login routes must survive stale bearer tokens

Affected routes include:

- `POST /api/quick-connect/initiate`
- `POST /api/quick-connect/redeem`
- `POST /api/auth/device-login`

These routes are intentionally usable before a device is authenticated. They are
the recovery path for a device that has never been paired, has been revoked, or
needs to sign in again with credentials.

The current global `DeviceTokenAuth` placement changes that behavior. Any
request carrying `Authorization: Bearer igd_...` is validated before the request
reaches the route handler. If the token hash no longer exists in the `devices`
table, the middleware returns `401 Unauthorized` immediately.

That is correct for protected API routes, but it is harmful for public pairing
and login routes. Many device clients attach their stored bearer token to every
API request automatically. After an admin revokes the device, that stored token
is stale. If the client then tries to re-pair through quick connect or sign in
through device login, the global middleware rejects the request before the
public handler can run. The user cannot recover until the client knows to strip
the header, which is brittle and pushes server auth semantics into client
cleanup behavior.

The expected behavior should be:

- Public pairing and device-login routes remain reachable even when the request
  contains a stale `igd_` bearer token.
- Protected routes still reject stale, revoked, or unknown `igd_` bearer tokens
  with `401 Unauthorized`.
- Non-Igloo bearer tokens, such as unrelated OAuth tokens, continue to be
  ignored by this middleware unless another route explicitly handles them.

There are two reasonable implementation shapes.

The cleaner approach is to avoid installing `DeviceTokenAuth` globally. Attach
it only to routes that can actually use device authentication, such as protected
API routes, the watch-room WebSocket route, and any public-but-auth-aware route
that intentionally accepts a device token. If `GET /api/auth/user` is expected
to work for device clients, keep bearer resolution on that route as well.

The smaller change is to keep the global middleware but explicitly bypass bearer
validation for public recovery routes. In that design, `DeviceTokenAuth` should
call the next handler without looking up or rejecting `igd_` tokens when the
request method and path match public recovery endpoints. This preserves the
current router shape, but the bypass list must stay synchronized with future
public auth and pairing routes.

The important detail is that invalid device tokens should only be fatal on
routes where device-token authentication is actually part of the route's auth
contract. A stale token attached to a public re-authentication request should be
treated as irrelevant, not as proof that the recovery request itself is
unauthorized.

Suggested tests:

- A revoked or unknown `Bearer igd_...` header does not prevent
  `/api/quick-connect/initiate` from reaching its normal request validation.
- A revoked or unknown `Bearer igd_...` header does not prevent
  `/api/quick-connect/redeem` from reaching its normal quick-connect logic.
- A revoked or unknown `Bearer igd_...` header does not prevent
  `/api/auth/device-login` from authenticating valid credentials.
- The same revoked or unknown token still receives `401 Unauthorized` on a
  protected route that accepts device bearer auth.
