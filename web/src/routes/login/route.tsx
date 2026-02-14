import { redirect, Outlet, createFileRoute } from "@tanstack/react-router";
import { fallback, zodSearchValidator } from "@tanstack/router-zod-adapter";
import * as z from "zod";
import { authUserQueryOpts } from "@/lib/query-opts";
import { getSafeRedirect } from "@/lib/redirect-utils";

const loginSearchValidator = z.object({
  redirect: fallback(z.string(), "/")
    .default("/")
    .transform((url: string) => getSafeRedirect(url)),
});

export const Route = createFileRoute("/login")({
  validateSearch: zodSearchValidator(loginSearchValidator),
  beforeLoad: async ({ context, search, location }) => {
    const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

    if (!res.error) {
      throw redirect({
        to: search.redirect,
        from: location.href,
      });
    }
  },
  component: () => <Outlet />,
});
