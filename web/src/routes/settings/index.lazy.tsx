import { createLazyFileRoute } from "@tanstack/react-router";
import { Link } from "@tanstack/react-router";

export const Route = createLazyFileRoute("/settings/")({
  component: RouteComponent,
});

function RouteComponent() {
  return (
    <main className='container mx-auto p-4'>
      <Link to='/settings/users' search={{ page: 1, limit: 10 }}>
        Go to Users
      </Link>

    </main>
  );
}
