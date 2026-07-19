import { redirect, createFileRoute } from "@tanstack/react-router";
import { authUserQueryOpts } from "@/lib/query-opts";
import { loginSearchSchema } from "@/lib/route-search";

export const Route = createFileRoute("/login")({
  validateSearch: loginSearchSchema,
  beforeLoad: async ({ context, search }) => {
    const res = await context.queryClient.fetchQuery(
      authUserQueryOpts({ revalidate: true }),
    );

    if (!res.error) {
      throw redirect({
        href: search.redirect,
      });
    }
  },
});
