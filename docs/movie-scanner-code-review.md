# Movie Scanner Code Review

## Summary

The scanner has one high-priority metadata-loss issue and six medium-priority reliability or user-experience issues. The most serious problem can permanently erase TMDB or manually entered metadata when a previously identified file changes. Other findings affect renamed unmatched movies, safe move detection, scan-status correctness, error-message safety, progress accuracy, and frontend error handling.

Priority meanings used in this review:

- **P1:** High priority. The issue can cause permanent data loss and should be fixed before release.
- **P2:** Medium priority. The issue breaks documented behavior, weakens safety, or creates a misleading user experience and should be fixed soon.

## Review comments

### 1. Preserve metadata when honoring a final match

- **Priority:** P1
- **Location:** [`server/cmd/internal/moviescanner/moviescanner_resolve.go:205`](../server/cmd/internal/moviescanner/moviescanner_resolve.go#L205)
- **Related requirement:** [`docs/movie-scanner-process.md`](movie-scanner-process.md#what-gets-written)

#### What happens

When a movie has already been matched successfully, or a user has identified it manually, that match is considered final. If the file's size or modification time later changes, the scanner processes the file again but deliberately skips the TMDB lookup. The skipped lookup supplies no metadata candidate, so the scanner rebuilds the movie record using only information parsed from the filename. During the database upsert, those filename-derived values and empty TMDB fields replace the richer metadata already stored for the movie.

#### Why this matters

This can erase the movie's title, TMDB and IMDB identifiers, poster and backdrop paths, synopsis, tagline, certification, ratings, and other metadata. It can also undo a user's manual identification even though the final match record itself remains in the database.

The loss is effectively permanent during normal scans. Because the match still says `matched` or `manual`, later scans continue to trust it and do not fetch the missing metadata again.

For example, changing the tags in a manually identified video can alter its file size and modification time. On the next scan, the movie may keep its library entry but lose the carefully selected identity and artwork.

#### Expected correction

When an existing final match is honored, the scanner must retain the movie's persisted identity and metadata while updating only file-derived information that genuinely changed, such as size, modification time, duration, or stream details. In particular, a manual identification must never be replaced or cleared by a routine rescan.

---

### 2. Reprocess renamed unmatched movies

- **Priority:** P2
- **Location:** [`server/cmd/internal/moviescanner/moviescanner_reconcile.go:76`](../server/cmd/internal/moviescanner/moviescanner_reconcile.go#L76)
- **Related requirement:** [`docs/movie-scanner-process.md`](movie-scanner-process.md#what-unmatched-means)

#### What happens

An unmatched movie can often be fixed by renaming its file to correct a misspelling or add a TMDB ID hint. The scanner detects the rename as a move and updates the existing movie row to point at the new path. It then removes that file from the set of changed files that still need processing.

Because the movie is removed from the processing set, the corrected filename is never parsed and no new TMDB lookup occurs. The existing final `unmatched` result remains attached to the movie. On the next scan, the recorded size and modification time match the file, so it is skipped again.

#### Why this matters

The documented recovery workflow—rename a badly named file and scan again—does not work. A user can correct the exact problem that prevented a match and still see the movie remain without its poster, cast, synopsis, and other TMDB metadata indefinitely.

For example, renaming `For.Yours.Eyes.Only.1981.mkv` to `For.Your.Eyes.Only.1981.mkv` correctly moves the existing database entry, but the scanner never searches TMDB using the corrected title.

#### Expected correction

After move detection updates the path of an unmatched movie, the renamed file must remain eligible for name parsing and matching. The scanner should preserve the existing movie row and related user data while replacing the stale unmatched result with the outcome of a lookup based on the new filename.

---

### 3. Require a complete walk before matching moves

- **Priority:** P2
- **Location:** [`server/cmd/internal/moviescanner/moviescanner_reconcile.go:40`](../server/cmd/internal/moviescanner/moviescanner_reconcile.go#L40)
- **Related requirement:** [`docs/movie-scanner-process.md`](movie-scanner-process.md#deletions-and-moves)

#### What happens

Move detection assumes that a database movie missing from the walk has actually disappeared from disk. It pairs such a movie with a newly discovered file when their byte sizes match unambiguously. However, move detection still runs when the filesystem walk was incomplete because a directory could not be read.

In that situation, a movie may be absent from the scan results even though its original file still exists in the unreadable directory. If a different, newly added movie has the same byte size, the scanner can mistake that new file for a move and repoint the old movie's database row to it.

#### Why this matters

This can attach the wrong identity and user data to an unrelated file. Watch progress, playlist membership, likes, and other relationships belonging to the original movie may now appear on the newcomer. Meanwhile, the original file still exists but is no longer represented correctly in the database.

The scanner already treats an incomplete walk as insufficient proof for deletion. Move detection needs the same level of caution because an incorrect move can corrupt associations rather than merely lose them.

#### Expected correction

Only perform move matching when the library walk completed successfully, or first confirm that each proposed source path is actually absent. A permission error, unavailable mount, or any result other than confirmed absence must be treated as unknown and must not be used as evidence of a move.

---

### 4. Initialize the pre-scan phase to `idle`

- **Priority:** P2
- **Location:** [`server/cmd/internal/moviescanner/moviescanner.go:170`](../server/cmd/internal/moviescanner/moviescanner.go#L170)
- **Related requirement:** [`docs/openapi.json`](openapi.json)

#### What happens

Before the first scan starts, the in-memory progress status has Go's zero value for its phase: an empty string. Calling the scan-status endpoint during that period therefore returns `"phase": ""` instead of the documented `"phase": "idle"`.

This can occur when status is requested immediately after startup, after a library is configured but before scanning begins, or during the short race between startup and the scan goroutine.

#### Why this matters

The API contract defines a fixed set of allowed phase values, and an empty string is not one of them. Frontend code uses the phase as a key for a human-readable label, so an unexpected empty phase can produce a missing label or undefined display state. API clients also should not have to invent behavior for a value the contract says cannot occur.

#### Expected correction

Create the scanner's initial progress status with `phase` set to `idle`. The status endpoint should always return one of the phase values declared in the OpenAPI contract, including before any scan has run.

---

### 5. Sanitize failed-scan status messages

- **Priority:** P2
- **Location:** [`server/cmd/internal/moviescanner/moviescanner.go:199`](../server/cmd/internal/moviescanner/moviescanner.go#L199)
- **Related requirement:** [`server/AGENTS.md`](../server/AGENTS.md)

#### What happens

When a fatal walk or database error stops a scan, the server logs the error and also places the raw `err.Error()` text into the public scan status. The status endpoint then returns that text to the web client.

Raw errors can contain internal filesystem paths, database or SQL details, driver messages, and other implementation information that was meant for server logs rather than an API response.

#### Why this matters

Exposing internal details unnecessarily reveals information about the server and can confuse users with messages they cannot act on. It also violates the backend rule that API responses use safe, client-facing messages while complete error details stay in structured logs for administrators and developers.

For example, a user needs to know that the library scan failed, but does not need a database statement or the full private path to the media library in the browser.

#### Expected correction

Continue logging the complete error on the server, but store a short, safe message in scan status, such as a general explanation that the library could not be read or the scan could not be completed. The client-facing message must not include raw paths, SQL, or other internal error details.

---

### 6. Count failed files as completed progress

- **Priority:** P2
- **Location:** [`server/cmd/internal/moviescanner/moviescanner.go:373`](../server/cmd/internal/moviescanner/moviescanner.go#L373)
- **Related requirement:** [`docs/movie-scanner-process.md`](movie-scanner-process.md#watching-a-scan)

#### What happens

Each file selected for processing contributes to the scan's `total`. A successfully processed file advances `processed`, but a file that fails during ffprobe or database persistence increments only `errors`. Even though the scanner has finished attempting that file, it is not counted as processed.

#### Why this matters

The live progress display can appear stuck below completion. If one out of ten files fails, the UI may remain at “9 of 10 movies processed” until the scan abruptly changes to its final state. With several failures, the gap is even more noticeable and can make a completed worker queue look stalled.

Here, `processed` represents completed attempts, not successful movies. Success and failure are already reported separately through the result counters.

#### Expected correction

For every non-cancellation file failure, increment both `errors` and `processed`. Cancellation should remain different because an interrupted file may not have completed its attempt. At the end of an uninterrupted processing phase, `processed` should equal `total` even when some individual files failed.

---

### 7. Render scan-status query failures

- **Priority:** P2
- **Location:** [`web/src/components/settings/MovieScanProgress.tsx:30`](../web/src/components/settings/MovieScanProgress.tsx#L30)
- **Related requirement:** [`docs/design-system.md`](design-system.md#error)

#### What happens

The scan-progress component returns nothing when the request fails at the network or query layer, when the API returns its failure envelope, or when no response data is available. As a result, the whole status area silently disappears.

#### Why this matters

An administrator cannot tell the difference between “there is no scan information to show” and “the server could not provide scan status.” The silent failure gives no explanation and no recovery action, so a temporary problem can look like normal behavior.

This also differs from the project's design-system pattern for inline query failures, which displays an accessible error and a retry control.

#### Expected correction

Use the query's error state together with the API failure envelope to render an inline error message. The error should use the established `role="alert"` styling and include a “Try again” action that calls the query's `refetch()` function. Continue hiding the component when no movie library is configured or when a valid response says the phase is `idle`; those are normal empty states rather than request failures.

## Overall assessment

Automated checks may pass, but the current behavior is not yet reliable enough for release. The metadata-loss issue should be addressed first because an ordinary file update can permanently remove curated data. The move-safety and unmatched-rename issues should follow because they can corrupt movie associations or prevent documented recovery. The remaining status and frontend issues are smaller in scope but are necessary for a safe, accurate, and understandable scan experience.
