# Repository Guidelines

## Project Structure & Module Organization

Igloo is split into `server/` and `web/`. The Go backend lives in `server/cmd/api` with internal packages under `server/cmd/internal`; SQL schema and queries are in `server/sqlc`. The React/Vite client lives in `web/src`, with route modules in `web/src/routes`, shared UI in `web/src/components`, hooks in `web/src/hooks`, and utilities in `web/src/lib`. Frontend unit tests are in `web/src/test`; Playwright specs are in `web/e2e`. Project notes and OpenAPI artifacts live in `docs/`.

## Core Principles

- Keep changes focused on the requested task.
- Avoid touching unrelated code.
- Preserve existing behavior unless the task explicitly requires changing it.
- Do not introduce large architectural changes without being asked.
- Apply the DRY principal where ever possible.
- Do not write small functions that can just be in line operations
- Prefer existing project patterns over new abstractions.
- Avoid excessive comments.
- Only add comments when the code is non-obvious or there is important context.
- Avoid adding dependencies unless they are necessary to complete the task.
- Do not add migrations or worry about backwards compatibility unless specifically requested. This application is not currently in production.
- This application is meant for self-hosting, not cloud-first platforms like AWS.
- Target platforms are Linux x64 and macOS ARM64.
- Windows, Docker deployment, and other Linux architectures are not currently contemplated.

## FFmpeg and ffprobe

- The FFmpeg and ffprobe binaries used by this project come from the Jellyfin project forks/builds.
- Be careful when changing media probing, transcoding, HLS, subtitle, or audio-track behavior.
- Video and audio playback changes must be tested carefully.

## Jellyfin Reference

- Jellyfin may be used as a reference for media playback, FFmpeg usage, HLS behavior, metadata fetching, movies, and TV shows.
- The local Jellyfin reference repository is located at:

  `/home/jose-ibanez/projects/jellyfin`

- Use Jellyfin as a reference, not as a source to blindly copy.
- The goal is to build a better, more accessible self-hosted media center, not to reimplement every Jellyfin feature.

## Server

- All handlers currently belong in the `main` package.
- Shared/global constants should be placed in:

  `/server/cmd/internal/helpers/constants.go`

- Prefer small, focused handlers.
- Check errors explicitly.
- Avoid inline error assignment in `if` statements.
- Prefer the Go standard library unless an existing dependency already solves the problem.
- Do not add new dependencies unless clearly necessary.

## Web

- Use React 19 features where appropriate.
- Screen reader support is essential.
- Always use Bun as the package manager.
- Use `@tanstack/react-router` for navigation.
- Use `@tanstack/react-query` for server-state/data fetching.
- Avoid manual memoization because this project uses the React Compiler.
- Do not write deeply nested ternary statements.
- Prefer clear conditional rendering over clever one-liners.
- Use shadcn/ui and Tailwind CSS for styling.

### Web File Organization

- Constants go in:

  `/web/src/lib/constants.ts`

- Helper functions and utilities go in:

  `/web/src/lib`

- Shared types go in:

  `/web/src/types`

- Components go in:

  `/web/src/components`

## Accessibility

- All interactive elements must have accessible names.
- Do not create unlabeled icon-only buttons.
- Keyboard navigation must work for interactive UI.
- Do not rely on hover-only interactions.
- Screen reader users must be able to understand and operate playback controls.
- Prefer semantic HTML where possible.
- Use ARIA only when semantic HTML is not enough.
- When adding dialogs, menus, popovers, or custom controls, verify focus behavior.
- Media controls, movie cards, navigation items, and settings controls must be understandable with a screen reader.

## TypeScript and React

- Use explicit shared types for API/domain models.
- Keep component props typed.
- Avoid unnecessary state.
- Prefer derived values over duplicated state.
- Use `async/await` instead of `.then()` and `.catch()` in application code.
- Validate external data where appropriate.
- Keep components focused and readable.

## Testing

- After code changes, run the most relevant tests for the affected area.
- For server changes, run the relevant Go tests.
- For web changes, run the relevant type checks, lint checks, and tests.
- Before completing large or risky changes, run the broader test suite when feasible.
- If a test fails, evaluate whether the failure is caused by the code change or by an existing issue.
- Do not change tests unless the test is incorrect or outdated and the application behavior is correct.
- Use Playwright for end-to-end testing of the web application.
- Features related to video playback, music playback, subtitles, audio tracks, transcoding, direct streaming, or HLS must be tested extensively.
