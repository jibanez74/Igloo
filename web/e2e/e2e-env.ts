export type E2EEnv = {
  baseURL: string;
  email: string;
  password: string;
};

export function readE2EEnv(): E2EEnv {
  return {
    baseURL: process.env.E2E_BASE_URL ?? "http://127.0.0.1:3000",
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}

export function apiURL(env: Pick<E2EEnv, "baseURL">, path: string) {
  return new URL(path, env.baseURL).toString();
}
