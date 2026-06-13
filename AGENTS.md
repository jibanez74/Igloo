# Repository Guidelines

## Project Overview

Igloo is a self-hosted media center with a Go backend and a React/Vite web client.

Igloo is designed for self-hosting on home servers and small personal infrastructure. Make technical decisions with local performance, predictable resource usage, and simple operations in mind. Do not assume deployment to large cloud platforms such as AWS, Azure, or Google Cloud unless explicitly requested.

The project is split into:

* `server/` — Go backend
* `web/` — React/Vite frontend
* `docs/` — project notes, OpenAPI artifacts, and supporting documentation

The application targets Linux x64 and macOS ARM64. Windows, Docker deployment, and other Linux architectures are not currently supported.

This project is not in production, so do not add migrations or preserve backwards compatibility unless specifically requested.

## Core Rules

* Keep changes focused on the requested task.
* Do not touch unrelated code.
* Preserve existing behavior unless the task requires changing it.
* Prefer existing project patterns over new abstractions.
* Avoid large architectural changes unless explicitly requested.
* Avoid unnecessary dependencies.
* Apply DRY where it improves clarity, but do not over-abstract.
* Do not extract tiny one-line helpers when inline code is clearer.
* Avoid excessive comments.
* Add comments only when the code is non-obvious or important context is needed.

## Project Structure

### Server

The Go backend lives in:

* `server/cmd/api`
* `server/cmd/internal`

SQL schema and queries live in:

* `server/sqlc`

Server rules:

* Handlers currently belong in the `main` package.
* Shared constants go in `server/cmd/internal/helpers/constants.go`.
* Prefer small, focused handlers.
* Check errors explicitly.
* Avoid inline error assignment in `if` statements.
* Prefer the Go standard library unless an existing dependency already solves the problem.
* Do not add new dependencies unless clearly necessary.

### Web

The React/Vite frontend lives in `web/src`.

Important directories:

* `web/src/routes` — route modules
* `web/src/components` — shared components
* `web/src/hooks` — React hooks
* `web/src/lib` — utilities, helpers, and constants
* `web/src/types` — shared TypeScript types
* `web/src/test` — frontend unit tests
* `web/e2e` — Playwright tests

Web rules:

* Always use Bun as the package manager.
* Use React 19 features where appropriate.
* Use `@tanstack/react-router` for navigation.
* Use `@tanstack/react-query` for server state and data fetching.
* Use shadcn/ui and Tailwind CSS for styling.
* Avoid manual memoization because the project uses the React Compiler.
* Avoid deeply nested ternary statements.
* Prefer clear conditional rendering.
* Use `async/await` instead of `.then()` and `.catch()` in application code.
* Keep component props typed.
* Use explicit shared types for API and domain models.
* Avoid unnecessary state.
* Prefer derived values over duplicated state.
* Validate external data where appropriate.

## Accessibility

Accessibility is a core requirement, not an enhancement.

* Use semantic HTML whenever possible.
* Use ARIA only when semantic HTML is not enough.
* All interactive elements must have accessible names.
* Do not create unlabeled icon-only buttons.
* Keyboard navigation must work for interactive UI.
* Do not rely on hover-only interactions.
* Screen reader users must be able to understand and operate the interface.
* Verify focus behavior when adding dialogs, menus, popovers, or custom controls.
* Media controls, movie cards, navigation items, and settings controls must be understandable with a screen reader.

## Media Playback

Igloo uses FFmpeg and ffprobe binaries from the Jellyfin project builds/forks.

Be careful when changing code related to:

* Media probing
* Direct streaming
* HLS
* Transcoding
* Subtitles
* Audio tracks
* Video playback
* Music playback

Playback-related changes must be tested carefully.

## Testing

Run the most relevant tests for the affected area after making changes.

* For server changes, run the relevant Go tests.
* For web changes, run the relevant type checks, lint checks, and tests.
* Use Playwright for end-to-end web testing.
* Before completing large or risky changes, run the broader test suite when feasible.
* If a test fails, determine whether the failure is caused by the change or by an existing issue.
* Do not change tests unless the test is incorrect or outdated and the application behavior is correct.
* Media playback, subtitles, audio tracks, transcoding, direct streaming, and HLS changes require extensive testing.
