import { redirect, Outlet, createFileRoute } from "@tanstack/react-router";
import * as z from "zod";
import { authUserQueryOpts } from "@/lib/query-opts";
import { getSafeRedirect } from "@/lib/redirect-utils";

const loginSearchValidator = z.object({
  redirect: z.string().catch("/")
    .default("/")
    .transform((url: string) => getSafeRedirect(url)),
});

export const Route = createFileRoute("/login")({
  validateSearch: loginSearchValidator,
  beforeLoad: async ({ context, search }) => {
    const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

    if (!res.error) {
      throw redirect({
        href: search.redirect,
      });
    }
  },
  component: () => <Outlet />,
});
