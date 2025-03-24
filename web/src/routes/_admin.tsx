import { Outlet, createFileRoute } from "@tanstack/solid-router";
import { authState } from "../stores/authStore";

export const Route = createFileRoute("/_admin")({
  beforeLoad: () => {
    if (!authState.user?.is_admin) {
      // throw new Error("403 - You do not have permission to access this page");
      console.log("user is not an admin");
    }
  },
  component: AdminLayout,
});

function AdminLayout() {
  return (
    <div>
      <h1>This is the admin layout</h1>

      <Outlet />
    </div>
  );
}
