import { Outlet, createFileRoute } from "@tanstack/react-router";
// import { authUserQueryOpts } from "@/lib/query-opts";
import Header from "@/components/Header";
import SideNav from "@/components/SideNav";

export const Route = createFileRoute("/_auth")({
  // beforeLoad: async ({ context, location }) => {
  //   const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

  //   if (res.error) {
  //     throw redirect({
  //       to: "/login",
  //       from: location.href,
  //     });
  //   }
  // },
  component: AuthLayout,
});

function AuthLayout() {
  return (
    <>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3
         focus:bg-amber-400 focus:text-slate-900 focus:px-3 focus:py-2 focus:rounded-md"
      >
        Skip to content
      </a>

      <Header />

      <div className="flex-1 flex overflow-hidden">
        <div className="w-full px-4 sm:px-6 lg:px-8 xl:px-12 py-6 grid md:grid-cols-[14rem_1fr] gap-6 lg:gap-8 flex-1">
          <SideNav />

          <main
            id="main"
            className="min-w-0 flex-1 overflow-y-auto pb-8"
          >
            <Outlet />
          </main>
        </div>
      </div>
    </>
  );
}
