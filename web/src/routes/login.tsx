import { redirect, createFileRoute } from "@tanstack/react-router";
import { z } from "zod/mini";
import { authUserQueryOpts } from "@/lib/query-opts";
import { getSafeRedirect } from "@/lib/redirect-utils";

const loginSearchValidator = z.object({
  redirect: z.pipe(
    z._default(z.catch(z.string(), "/movies"), "/movies"),
    z.transform((url: string) => getSafeRedirect(url, "/movies")),
  ),
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
});
