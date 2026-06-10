import { redirect, createFileRoute } from "@tanstack/react-router";
import { authUserGuardQueryOpts } from "@/lib/query-opts";
import { loginSearchSchema } from "@/types/route-search";

export const Route = createFileRoute("/login")({
  validateSearch: loginSearchSchema,
  beforeLoad: async ({ context, search }) => {
    const res = await context.queryClient.fetchQuery(authUserGuardQueryOpts());

    if (!res.error) {
      throw redirect({
        href: search.redirect,
      });
    }
  },
});
