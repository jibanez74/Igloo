// Fixture builders for the Go `sql.Null*` wire shapes and the authenticated
// user payload, both of which appear in nearly every mocked API response.

export function nullableString(value = "") {
  return {
    String: value,
    Valid: value.length > 0,
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

/** A `GET /api/auth/user` payload. Override any field via `overrides`. */
export function authUser(overrides: Record<string, unknown> = {}) {
  return {
    error: false,
    data: {
      user: {
        id: 1,
        name: "Test User",
        email: "test@example.com",
        is_admin: false,
        avatar: null,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
        ...overrides,
      },
    },
  };
}
