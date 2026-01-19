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
         focus:rounded-md focus:bg-amber-400 focus:px-3 focus:py-2 focus:text-slate-900"
      >
        Skip to content
      </a>

      <Header />

      <div className="flex flex-1 overflow-hidden">
        <div className="grid w-full flex-1 gap-6 px-4 py-6 sm:px-6 md:grid-cols-[14rem_1fr] lg:gap-8 lg:px-8 xl:px-12">
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
