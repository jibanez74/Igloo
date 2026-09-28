import { expect, type APIRequestContext } from "@playwright/test";
import type { AdminUserType } from "../src/types";
import { readJSON } from "./e2e-api";

// Disposable accounts for specs that drive the Users API. Every helper runs on
// a request context that already holds an admin session.

type NewUser = {
  name: string;
  email: string;
  password: string;
  is_admin?: boolean;
};

export async function createUser(request: APIRequestContext, user: NewUser) {
  const response = await request.post("/api/admin/users", {
    data: { is_admin: false, ...user },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(201);

  const body = await readJSON<{ user: AdminUserType }>(response);
  expect(body.error, body.message).toBe(false);
  return body.data!.user;
}

export async function fetchAdminUsers(request: APIRequestContext) {
  const response = await request.get("/api/admin/users", {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<{ users: AdminUserType[] }>(response);
  expect(body.error, body.message).toBe(false);
  return body.data?.users ?? [];
}

/**
 * Deletes one account. `allowMissing` also accepts 404, for `finally` cleanup
 * that must not mask the test's own failure when the user is already gone.
 */
export async function deleteUser(
  request: APIRequestContext,
  userId: number,
  { allowMissing = false }: { allowMissing?: boolean } = {},
) {
  const response = await request.delete(`/api/admin/users/${userId}`, {
    failOnStatusCode: false,
  });
  expect(allowMissing ? [200, 404] : [200]).toContain(response.status());
}

/** Deletes every account whose email starts with `prefix`. */
export async function deleteUsersByEmailPrefix(
  request: APIRequestContext,
  prefix: string,
) {
  for (const user of await fetchAdminUsers(request)) {
    if (user.email.startsWith(prefix)) {
      await deleteUser(request, user.id);
    }
  }
}
