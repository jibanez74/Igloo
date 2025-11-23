import { createLazyFileRoute } from "@tanstack/react-router";

export const Route = createLazyFileRoute("/login/")({
  component: LoginPage,
});

function LoginPage() {
  return (
    <div>
      <h1>Hello Login Page</h1>
    </div>
  );
}
