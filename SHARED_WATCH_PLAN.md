# Shared Watch Feature Plan

## Overview

This document defines the implementation plan for the new shared watch feature in Igloo.

The goal is to let a user create a private watch room for a movie, invite existing users from the system, and watch together in sync. All invited users in the room can control playback with play, pause, rewind, and fast-forward actions, while the room creator remains the only user allowed to delete the room.

The implementation must:

- fit the current Go + React architecture
- preserve the current playback flow for non-room playback
- use websockets for realtime sync
- support invited users only
- pre-warm HLS playback when needed
- avoid breaking any existing tests
- treat existing tests as fixed constraints

This plan is intentionally detailed so implementation can proceed incrementally and safely.

    ---

## Product Goals

The shared watch feature should improve the local-first movie playback experience without expanding the product outside its current scope.

Primary goals:

- let a user create a room from a movie details page
- let the room creator invite other existing users from the database
- let invited users discover that room easily from the home page
- let invited users join and watch the same movie together
- keep all participants synchronized during playback
- use the creator's playback presets for the room
- allow only the creator to delete the room
- make HLS playback smoother by starting FFmpeg work as soon as the room is created

Non-goals for this first version:

- editing room settings after creation
- open public room links for anonymous users
- cross-media support beyond movies
- chat, reactions, or social features unrelated to shared playback
- changing ownership of a room

---

## Final User Experience

### Room Creation

The room creation flow starts from the existing `Watch Together` option in the movie details page action menu.

When the user selects that option:

- a dialog opens
- the selected movie is already implied by the current page
- the dialog shows the current playback presets
- the user can choose invited participants from existing users in the database
- the user confirms room creation

If room creation succeeds:

- the backend stores the room and membership list
- the backend starts HLS warm-up immediately if the room uses an HLS mode
- the frontend shows success feedback
- the creator can proceed into the room

### Room Discovery

Invited users should not need a direct URL to discover a room.

The authenticated home page should include a prominent section near the top that displays rooms available to the current user.  This section must follow current styles and not break the look of the application.

Each room card should include:

- movie poster
- movie title
- room owner
- invited users or members
- a join button
- an owner-only delete action

This section should be visually important and easy to scan, since it is the main room entry point for invited users.

### Room Playback

When a user joins a room:

- the room page loads room metadata
- the room page verifies the user is a member
- the room page connects to the websocket
- the video player uses the room's playback presets
- local playback controls send websocket messages to the room
- incoming websocket messages update the local player to stay in sync

All room members can control:

- play
- pause
- rewind
- fast-forward
- seeking

Only the room creator can:

- delete the room

### Room Deletion

The creator must have two clear ways to delete the room:

- from the room card on the home page
- from the room page itself

When a room is deleted:

- the backend deletes the room and membership rows
- the backend stops and cleans up the room HLS session if one exists
- connected room users receive a websocket room-deleted event
- users are redirected out of the room with a clear message

---

## Functional Requirements

### Room Membership

- A room creator must select invited users at creation time.
- Invited users must already exist in the `users` table.
- The room owner must also be a room member.
- Only room members can access room details.
- Only room members can join the room websocket.
- Non-members must receive a clear forbidden response.

### Room Mutability

- Rooms are immutable after creation except for deletion.
- Playback presets are fixed when the room is created.
- Invited users are fixed when the room is created.
- The room owner cannot edit the room later in this first version.

### Playback Authority

- All room members can control playback.
- The server should maintain the authoritative playback state for the room.
- Clients should follow server updates and correct drift carefully.

### Playback Presets

The creator chooses the room playback presets at creation time. These settings must be stored with the room:

- playback mode
- selected audio track
- selected subtitle track

These settings determine the stream URLs and playback behavior for all room members.

### HLS Warm-Up

If the room uses an HLS mode instead of direct playback:

- the backend must start the room-specific HLS session immediately on room creation
- FFmpeg should begin producing assets before users join
- this should reduce playback startup latency for participants

### Home Page Visibility

The home page room section should show rooms relevant to the current user:

- rooms the user owns
- rooms the user was invited to

The section should not show rooms the user is not part of.

This section should be completely hidden if there are no rooms available.

### Accessibility

The implementation must preserve accessibility expectations:

- keyboard access for creation, joining, and deletion
- semantic buttons and dialogs
- clear screen reader labels
- live announcements for key room sync changes when appropriate
- clear forbidden and deleted-room states

---

## Constraints

### Existing Playback Must Not Regress

The standard movie playback route must continue to work exactly as it does today for users who are not using shared watch.

Do not introduce unnecessary changes to existing playback APIs or route behavior when a new room-specific flow can be added separately.

### Existing Tests Are Fixed Constraints

This is a hard rule for implementation:

- existing tests must not be weakened or rewritten to accommodate regressions
- if an existing test breaks, the implementation must be adjusted
- new code should be isolated enough that existing behavior remains stable

### Architecture Constraints

Follow existing repository conventions:

- backend HTTP handlers in `server/cmd/api/`
- middleware in `server/cmd/api/middleware.go`
- sqlc for database access
- shared frontend API helpers in `web/src/lib/api.ts`
- shared query options in `web/src/lib/query-opts.ts`
- types in `web/src/types/`

---

## High-Level Architecture

The feature should be split into three layers:

1. Persistent room data in SQLite
2. REST APIs for room lifecycle and discovery
3. Websocket-based realtime playback synchronization

This separation keeps the design clear:

- SQLite stores the durable room and membership data
- REST handles creation, listing, joining, and deletion
- websockets handle ephemeral realtime events

The room should be treated as a first-class feature, but it should remain layered on top of the existing movie playback implementation instead of replacing it.

---

## Backend Design

## Database Model

### `watch_rooms`

This table will store one row per room.

Suggested fields:

- `id`
- `owner_user_id`
- `movie_id`
- `playback_mode`
- `audio_track`
- `subtitle_track`
- `created_at`
- `updated_at`

### `watch_room_members`

This table will store invited users for each room.

Suggested fields:

- `id`
- `room_id`
- `user_id`
- `created_at`
- `updated_at`

Important rules:

- the owner must also be inserted as a member
- there should be a uniqueness constraint on `(room_id, user_id)`
- deleting a room should cascade to its members

### Foreign Keys

Use foreign keys to preserve integrity:

- `owner_user_id` references `users(id)`
- `movie_id` references `movies(id)`
- `room_id` references `watch_rooms(id)`
- `user_id` references `users(id)`

### Why No Edit Tables or Event History Table in V1

We are not supporting room editing in this version, so there is no need for a room revisions model.

We are also not planning a persistent event log table for playback events in v1 because:

- websocket sync state is transient
- persistent event history is not required for the core feature
- we should keep the first iteration small and reliable

---

## SQLC Query Plan

Add new query files for room operations, with focused queries that match existing project conventions.

Required query groups:

- create room
- delete room
- fetch room by id
- fetch visible rooms for a user
- fetch room members
- verify room owner
- verify room membership
- insert member rows
- validate user IDs exist

Representative operations:

- `CreateWatchRoom`
- `AddWatchRoomMember`
- `GetWatchRoomByID`
- `GetWatchRoomsForUser`
- `GetWatchRoomMembers`
- `IsWatchRoomOwner`
- `IsWatchRoomMember`
- `DeleteWatchRoom`
- `CountUsersByIDs`

The create flow will likely require a transaction so that:

- the room is created
- the owner and invited members are inserted
- failure rolls the entire operation back

That transaction should happen in the API handler or a focused helper in the backend layer, while keeping handlers thin.

---

## REST API Plan

All room routes should be authenticated.

Suggested route group:

- `GET /api/watch-rooms`
- `GET /api/watch-rooms/{id}`
- `POST /api/watch-rooms`
- `POST /api/watch-rooms/{id}/join`
- `DELETE /api/watch-rooms/{id}`

### `GET /api/watch-rooms`

Purpose:

- return all rooms visible to the current authenticated user
- primarily used by the home page shared-room section

Rules:

- include rooms the user owns
- include rooms the user is invited to
- exclude rooms unrelated to the user

Suggested response payload:

- room id
- movie id
- movie title
- movie poster
- owner summary
- member summaries
- playback mode summary
- whether current user is owner

### `GET /api/watch-rooms/{id}`

Purpose:

- return room details for a specific room
- used by the room page loader before websocket connection

Rules:

- only members can access
- non-members get `403`
- unknown room gets `404`

Suggested payload:

- room metadata
- movie metadata needed by the room page
- member list
- ownership flag
- fixed playback settings

### `POST /api/watch-rooms`

Purpose:

- create a new room with invited users and fixed playback presets

Request body should include:

- `movie_id`
- `mode`
- `audio_track`
- `subtitle_track`
- `invited_user_ids`

Validation rules:

- movie must exist
- mode must be valid
- audio track must be valid for the movie
- subtitle track must be valid or nullable
- invited user IDs must all exist
- duplicates should be rejected or normalized
- the creator should not need to manually invite themselves

Success behavior:

- create room row
- add owner as member
- add invited users as members
- if mode is HLS, start room-specific HLS warm-up

### `POST /api/watch-rooms/{id}/join`

Purpose:

- validate that the current user is a member
- return the data required to enter the room safely

This route may seem redundant with `GET /api/watch-rooms/{id}`, but it gives us a clean place for join-specific behavior later if needed, such as:

- recording presence
- issuing websocket tokens if ever needed
- returning ephemeral connection metadata

For v1, it can be a simple membership-validated handshake endpoint.

### `DELETE /api/watch-rooms/{id}`

Purpose:

- delete a room permanently

Rules:

- only the owner can delete the room
- invited members cannot delete it

Deletion behavior:

- verify owner
- delete room row
- cascade delete member rows
- stop and clean up room HLS session
- notify connected websocket clients that the room is gone

---

## User Lookup API Plan

The room creator needs a way to select invited users from existing users.

There does not appear to be a current user listing API suitable for this feature, so we should add a small authenticated endpoint for this purpose.

Suggested route:

- `GET /api/users`

Possible shape:

- allow optional search filtering by name or email
- exclude the current user in the response
- keep the response small and only expose fields needed for selection

Suggested response fields:

- `id`
- `name`
- `email`
- `avatar`

This endpoint is only for room invite selection and similar internal UI features. It should not expose extra data beyond what is necessary.

---

## Websocket Plan

## Why Websockets

Shared watch requires low-latency sync for:

- play
- pause
- seek
- member presence
- room deletion notification

This makes websockets the right fit for the realtime layer.

## Required Backend Package

Add:

- `github.com/gorilla/websocket`

Why this package:

- mature and widely used
- works well with `net/http` and `chi`
- sufficient for room-level synchronization and presence
- no need for a more complex realtime framework

No additional frontend websocket package is needed because the browser-native `WebSocket` API is sufficient.

## Websocket Endpoint

Suggested route:

- `GET /api/watch-rooms/{id}/ws`

Rules:

- user must be authenticated
- user must be a room member
- non-members must be rejected before upgrading the connection

## Room Hub

Add an in-memory room hub owned by the backend application process.

Responsibilities:

- track active room connections
- track per-room connected users
- maintain canonical playback state
- broadcast updates
- notify clients when the room is deleted

This state does not need to be persisted in SQLite because it is transient and tied to live connections.

## Canonical Playback State

Each active room should maintain an authoritative playback state:

- room id
- whether playback is paused
- current playback position in seconds
- last update timestamp

This lets the server determine the current shared position and send accurate snapshots to late joiners.

## Client Events

Planned incoming websocket events:

- `join`
- `play`
- `pause`
- `seek`
- `ping` or heartbeat

## Server Events

Planned outgoing websocket events:

- `room_snapshot`
- `playback_changed`
- `member_joined`
- `member_left`
- `room_deleted`
- optional `sync_required`

## Sync Strategy

The server should be authoritative, but the client should not overreact to tiny timing differences.

Recommended behavior:

- accept a local drift threshold
- only force seeking when the drift is meaningful
- avoid replaying the same sync event repeatedly
- treat pause and play changes as immediate
- treat large seek jumps as authoritative

This will help preserve a smoother playback experience.

---

## HLS Warm-Up Plan

## Requirement

If a room uses HLS playback, FFmpeg should start generating output immediately after room creation so playback is smoother once users join.

## Existing Behavior

The current HLS flow creates sessions lazily when the manifest is requested.

For shared watch, this is not enough because the first participant would still pay the startup cost.

## New Behavior

When a room is created with a non-direct mode:

- create the room in the database
- immediately start the room HLS session at `start=0`
- keep that session alive under a room-aware cache key

## Room-Specific HLS Keying

This is important.

Current HLS session cache keys are based on movie and stream settings. That is not sufficient for shared watch because collisions could occur between:

- standard individual playback
- one shared room
- multiple shared rooms for the same movie and settings

The shared watch implementation should use a room-aware key for room playback sessions.

Example concept:

- personal playback: existing keying behavior remains unchanged
- room playback: room-specific key includes room id

This ensures:

- room HLS lifecycle is isolated
- cleanup is straightforward
- shared watch does not interfere with normal playback

## HLS Cleanup on Room Delete

When the room is deleted:

- stop the associated FFmpeg process if it is still running
- remove its temp directory
- remove its cache entry

This cleanup should happen even if no users are currently connected.

## Subtitle Warm-Up

This is optional in the first iteration, but worth noting.

If a subtitle track is selected and requires conversion to WebVTT, pre-warming subtitle extraction could further reduce first-play delays. This is lower priority than HLS warm-up and can be deferred unless room startup testing shows it is needed.

---

## Frontend Plan

## Create Room Entry Point

Replace the `Coming soon` behavior in the movie details action menu with a real create-room flow.

Likely file:

- `web/src/components/MovieDetailsHeroActions.tsx`

The action should open a room creation dialog instead of a toast.

## Create Room Dialog

This dialog should:

- present the movie context clearly
- show the chosen playback settings
- let the creator search and select invited users
- validate required inputs
- submit room creation

Suggested responsibilities:

- load available users through the new user lookup API
- prevent duplicate selections
- allow removing selected invitees before submission
- surface API validation errors clearly

Accessibility requirements:

- proper dialog semantics
- labelled inputs
- keyboard navigation for user selection
- clear screen reader descriptions

## Home Page Room Section

Add a prominent section near the top of the authenticated home page.

This section should:

- load visible rooms for the current user
- render nothing if there are no rooms, unless design prefers an empty-state panel
- show room cards with poster, title, participants, and join button
- show delete action only for the owner

Suggested card content:

- movie poster
- movie title
- owner label
- list or avatar row of members
- join button
- delete button for owner

The layout should feel intentionally important since this is the primary discovery surface for invited users.

## Room Page

Add a dedicated route for room playback.

Suggested route concept:

- `/_auth/watch-rooms/$id`

The room page should:

- load room details first
- reject non-members cleanly
- connect websocket after successful load
- display the room movie and member context
- render the playback UI in room mode
- show owner-only deletion action

## Reuse of Existing Player

We should reuse as much of the current movie playback logic as possible.

However, we should avoid forcing major changes into the existing non-room play route if a room-aware wrapper or shared player logic extraction keeps the implementation safer.

Preferred approach:

- preserve the current personal playback route behavior
- extract shared player logic only where it clearly reduces duplication
- introduce room-specific orchestration around playback control

## Room Deletion UX

The creator should be able to delete the room from:

- the home page room card
- the room page header or banner

Deletion should use a confirmation dialog.

After delete succeeds:

- local room list updates
- any room page navigates away
- any connected participant receives a room deleted event and exits gracefully

---

## API and Query Client Integration

Add new frontend API helpers in:

- `web/src/lib/api.ts`

Expected functions:

- `getWatchRooms`
- `getWatchRoom`
- `createWatchRoom`
- `joinWatchRoom`
- `deleteWatchRoom`
- `getUsers`

Add corresponding query options in:

- `web/src/lib/query-opts.ts`

Expected query patterns:

- room list for the home page
- single room details for the room route
- users list or user search for the creation dialog

Query invalidation should be narrow and predictable:

- invalidate room list after create
- invalidate room list and room details after delete
- avoid disturbing existing movie queries unless truly necessary

---

## Permissions Model

The permissions model must remain simple and predictable.

### Owner

The room owner can:

- create the room
- join the room
- control playback
- delete the room

The room owner cannot:

- edit the room after creation in v1

### Invited Member

An invited member can:

- see the room on the home page
- join the room
- connect to the websocket
- control playback

An invited member cannot:

- delete the room
- edit the room

### Non-Member

A non-member cannot:

- see the room in their home page room section
- fetch room details
- join the room
- connect to the room websocket

---

## Error and Edge Case Handling

The implementation should define predictable behavior for important edge cases.

### Invalid Invited Users

If the creator submits user IDs that do not exist:

- room creation should fail cleanly
- no partial room should be created

### Duplicate Invited Users

If duplicate user IDs are submitted:

- normalize them before insertion or reject validation cleanly

### Owner Included in Invites

If the creator includes themselves in the invited user list:

- do not duplicate the owner membership row

### Deleted Room During Playback

If the owner deletes the room while others are connected:

- websocket clients receive `room_deleted`
- clients stop room playback flow gracefully
- UI redirects users out of the room

### HLS Warm-Up Failure

If room creation succeeds but HLS warm-up fails:

- decide whether room creation should fail atomically or succeed with degraded startup

Recommended choice for v1:

- fail room creation if required HLS warm-up cannot start

Reason:

- the room should not be created in a partially broken state if the selected playback mode depends on a transcode session that cannot start

### Direct Playback Rooms

If the room uses direct playback:

- no HLS warm-up runs
- room creation should still succeed

### User Tries to Access Room by URL Without Invite

- return `403` from backend
- show accessible forbidden state in frontend

---

## Testing Strategy

Testing is a mandatory part of this feature plan.

The feature touches playback, routing, authorization, database persistence, and realtime communication. Because of that, it must be implemented with strong regression discipline.

## Core Testing Rule

Existing tests must not be weakened.

If an existing test breaks:

- fix the implementation
- do not loosen the test
- do not rewrite the test to accept a regression

## Backend Test Plan

Add backend tests for room persistence and API behavior.

### Database and API Tests

Test cases should include:

- room creation succeeds with valid movie and invited users
- room creation fails when an invited user does not exist
- room creation inserts owner membership automatically
- room creation deduplicates or rejects duplicate invite IDs correctly
- room listing returns owner rooms
- room listing returns invited rooms
- room listing excludes unrelated rooms
- room details returns `403` for non-members
- room details returns `404` for missing rooms
- join returns success for members
- join returns `403` for non-members
- delete returns `403` for invited non-owner members
- delete returns success for owner
- delete removes room rows
- delete removes membership rows

### HLS Warm-Up Tests

Test cases should include:

- HLS room creation starts a room-specific HLS session
- direct-play room creation does not start HLS warm-up
- room deletion cleans up the room HLS session
- room HLS cache key does not collide with standard playback keying

### Websocket Backend Tests

Test cases should include:

- room member can connect successfully
- non-member cannot connect
- `play` event updates canonical room state
- `pause` event broadcasts correctly
- `seek` event broadcasts correctly
- member join and leave presence events are emitted
- room deletion broadcasts room-deleted event to all connected members

These tests can be introduced as focused handler or integration tests without changing unrelated backend tests.

## Frontend Test Plan

Add frontend tests centered on user behavior.

### Creation Flow Tests

Test cases should include:

- watch together action opens the create-room dialog
- user selection UI renders fetched users
- duplicate invite selection is prevented
- create submission surfaces validation errors
- create success closes or advances appropriately

### Home Page Tests

Test cases should include:

- room section renders when rooms exist
- room section does not regress the rest of the home page
- room card shows poster, title, participants, and join button
- owner sees delete action
- invited non-owner does not see delete action

### Room Page Tests

Test cases should include:

- invited member can load the room page
- forbidden state is shown accessibly for non-members
- room deleted event removes the user from the page flow gracefully
- room controls send sync actions in room mode

## Regression Test Plan

Before merging any implementation phase:

- run all existing backend tests
- run all existing frontend tests
- verify normal movie playback still works
- verify movie details page still works without shared room usage
- verify home page still works when there are zero shared rooms

New tests should be additive. Existing tests should remain semantically intact.

---

## Implementation Phases

The feature should be implemented incrementally to reduce risk and keep regressions contained.

## Phase 1: Schema and Query Layer

Deliverables:

- schema updates for `watch_rooms` and `watch_room_members`
- new sqlc queries
- generated database code

Validation:

- schema initializes successfully
- query layer compiles
- new database tests pass
- existing backend tests remain green

## Phase 2: Room REST APIs

Before implementing, read server/AGENTS.md to understand the rules for the server.

Deliverables:

- room list endpoint
- room details endpoint
- room create endpoint
- room join endpoint
- room delete endpoint
- user lookup endpoint

Validation:

- API handlers return expected responses
- authorization rules are enforced
- existing API tests remain green

## Phase 3: HLS Warm-Up

Deliverables:

- room-aware HLS session keying
- eager HLS startup on room creation
- room deletion cleanup

Validation:

- warm-up tests pass
- normal HLS playback behavior remains unchanged

## Phase 4: Create Room Dialog

Deliverables:

- real create-room dialog from movie details
- user selection flow
- room creation mutation and feedback

Validation:

- creation flow tests pass
- movie details tests remain green


## Phase 5: Home Page Discovery

Deliverables:

- shared-room section at top of home page
- room cards
- join and delete actions
- Room section should be hidden if there are no rooms available.

Validation:

- frontend room list behavior works
- home page layout and accessibility stay solid
- existing home page tests remain green

## Phase 6: Room Route and Websocket Sync

Deliverables:

- room playback route
- websocket connection flow
- room-aware synchronized playback controls

Validation:

- realtime sync tests pass
- normal movie player remains stable

## Phase 7: Room Deletion Flow

Deliverables:

- owner delete action on room card
- owner delete action on room page
- confirmation dialog
- redirect and websocket notification behavior

Validation:

- delete flow tests pass
- cleanup behavior is reliable

## Phase 8: Full Regression and Polish

Deliverables:

- run full backend and frontend suites
- resolve regressions by fixing implementation
- accessibility review
- error message review

Validation:

- all existing tests pass
- all new tests pass
- feature works end-to-end

---

## Package Changes

Required backend package addition:

- `github.com/gorilla/websocket`

No new frontend websocket package is required.

No additional package is required specifically for room deletion.

The current frontend stack already includes the dialog and UI primitives needed for:

- create-room dialog
- delete confirmation dialog
- room cards
- buttons and menus

---

## File Areas Likely to Change

This is not a final file list, but it identifies the most likely implementation areas.

### Backend

- `server/cmd/api/schema.sql`
- new sql query files under `server/sqlc` or current sqlc query location
- generated files under `server/cmd/internal/database/`
- `server/cmd/api/main.go`
- new room handlers under `server/cmd/api/`
- possible room hub and websocket files under `server/cmd/api/`
- HLS session files under `server/cmd/api/`

### Frontend

- `web/src/components/MovieDetailsHeroActions.tsx`
- home page route/component files
- new room dialog component
- new room card component
- new room route
- `web/src/lib/api.ts`
- `web/src/lib/query-opts.ts`
- new shared watch types in `web/src/types/`

---

## Success Criteria

The feature is considered complete when:

- a user can create a room from a movie details page
- invited users must already exist in the database
- invited users can discover the room from the top home page section
- invited users can join and watch the movie together
- playback remains synchronized via websocket events
- the creator's playback presets are used for the room
- only the creator can delete the room
- HLS rooms start FFmpeg warm-up immediately on creation
- deleting a room cleans up associated room playback resources
- all existing tests still pass
- new tests cover the critical room flows

---

## Final Implementation Guidance

Keep the implementation incremental, scoped, and compatible with the current architecture.

Do not force major rewrites of existing playback paths unless absolutely necessary.

Prefer introducing new room-specific layers over changing existing stable contracts.

When in doubt:

- preserve current behavior
- add new behavior in parallel
- test aggressively
- fix regressions in code, not by weakening tests
