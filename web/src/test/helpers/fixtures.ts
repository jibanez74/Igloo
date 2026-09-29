// Fixture builders for the Go `sql.Null*` wire shapes, the authenticated user
// payload and the server playback settings, which appear across the mocked API
// responses.

import type { AuthUser, PlaybackSettingsType } from "@/types";

// null or "" is the invalid (SQL NULL) form; a test that needs a valid empty
// string writes the literal.
export function nullableString(value: string | null = "") {
  return {
    String: value ?? "",
    Valid: value != null && value.length > 0,
  };
}

export function nullableInt64(value: number | null = null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

export function nullableFloat64(value: number | null = null) {
  return {
    Float64: value ?? 0,
    Valid: value != null,
  };
}

/** The authenticated user object. Override any field via `overrides`. */
export function authUserData(overrides: Partial<AuthUser> = {}): AuthUser {
  return {
    id: 1,
    name: "Test User",
    email: "test@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

/** A `GET /api/auth/user` payload. Override any user field via `overrides`. */
export function authUser(overrides: Partial<AuthUser> = {}) {
  return {
    error: false,
    data: { user: authUserData(overrides) },
  };
}

/**
 * A `GET /api/settings/playback` settings object: a two-profile catalog on a
 * CPU server, which the server caps at 1080p. Override any field via
 * `overrides`.
 */
export function playbackSettings(
  overrides: Partial<PlaybackSettingsType> = {},
): PlaybackSettingsType {
  return {
    profiles: [
      { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
      { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
    ],
    server_upload_mbps: null,
    hardware_acceleration_device: "cpu",
    effective_hardware_acceleration_device: "cpu",
    hardware_fallback_reason: "",
    max_transcode_height: 1080,
    ...overrides,
  };
}
