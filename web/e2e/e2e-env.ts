export type E2EEnv = {
  baseURL: string;
  email: string;
  password: string;
};

export type Credentials = Pick<E2EEnv, "email" | "password">;

/**
 * An integer environment variable of at least `min`. Unset or invalid values
 * give `fallback`.
 */
export function intEnv(name: string, fallback: number, min?: number): number;
export function intEnv(name: string, fallback?: undefined, min?: number): number | undefined;
export function intEnv(name: string, fallback?: number, min = 1) {
  const raw = process.env[name];
  if (!raw) return fallback;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed >= min ? parsed : fallback;
}

export function readE2EEnv(): E2EEnv {
  return {
    baseURL:
      process.env.E2E_BASE_URL ??
      `http://127.0.0.1:${intEnv("E2E_WEB_PORT", 3000)}`,
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}
