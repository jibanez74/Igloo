import { useState, useTransition } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import { showSuccess, showError } from "@/lib/toast-helpers";
import { Snowflake, Mail, Lock, Eye, EyeOff, LogIn } from "lucide-react";
import { login } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import loginBg from "@/assets/images/login-bg.jpg";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";

const pageTitle = "Sign In - Igloo";
const pageDescription = "Sign in to access your personal Igloo media library.";

export const Route = createLazyFileRoute("/login/")({
  component: LoginPage,
});

function LoginPage() {
  const [isPending, startTransition] = useTransition();
  const [showPassword, setShowPassword] = useState(false);
  const navigate = Route.useNavigate();
  const { redirect } = Route.useSearch();
  const { queryClient } = Route.useRouteContext();

  const loginHandler = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    startTransition(async () => {
      const formData = new FormData(e.currentTarget);
      const email = formData.get("email") as string;
      const password = formData.get("password") as string;

      const res = await login(email, password);

      if (res.error) {
        showError(
          "Login failed",
          res.message || "An error occurred during login"
        );

        return;
      }

      showSuccess("Welcome back!", res.message || "Login successful");

      await queryClient.fetchQuery(authUserQueryOpts());
      await queryClient.invalidateQueries();

      navigate({
        to: redirect,
        from: "/login",
        replace: true,
      });
    });
  };

  return (
    <div className="h-full bg-slate-900 text-slate-100 antialiased">
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <div className="relative min-h-screen">
        {/* Background image */}
        <div
          className="absolute inset-0 bg-cover bg-center bg-no-repeat"
          style={{ backgroundImage: `url(${loginBg})` }}
        />
        {/* Dark overlay */}
        <div className="absolute inset-0 bg-slate-950/70" />

        <main className="relative z-10 flex min-h-screen items-center justify-center px-4">
          <Card
            className="w-full max-w-md animate-in border-slate-800 bg-slate-900/80 shadow-xl
               backdrop-blur-sm duration-500 fade-in slide-in-from-bottom-2"
          >
            <CardHeader className="pb-2 text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-slate-800">
                <Snowflake
                  className="size-5 text-amber-400"
                  aria-hidden="true"
                />
              </div>
              <CardTitle className="text-2xl font-semibold tracking-tight text-slate-100">
                Welcome to Igloo
              </CardTitle>
              <CardDescription className="text-slate-400">
                Sign in to access your private media library.
              </CardDescription>
            </CardHeader>

            <CardContent>
              <form onSubmit={loginHandler} className="space-y-4">
                {/* Email field */}
                <div className="space-y-1">
                  <Label htmlFor="email">Email</Label>
                  <div className="relative">
                    <Mail
                      className="absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-slate-400"
                      aria-hidden="true"
                    />
                    <Input
                      autoFocus
                      type="email"
                      id="email"
                      name="email"
                      inputMode="email"
                      autoComplete="username"
                      required
                      className="pl-10"
                      disabled={isPending}
                    />
                  </div>
                </div>

                {/* Password field */}
                <div className="space-y-1">
                  <Label htmlFor="password">Password</Label>
                  <div className="relative">
                    <Lock
                      className="absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-slate-400"
                      aria-hidden="true"
                    />
                    <Input
                      type={showPassword ? "text" : "password"}
                      minLength={9}
                      maxLength={128}
                      id="password"
                      name="password"
                      required
                      className="px-10"
                      disabled={isPending}
                    />
                    <button
                      type="button"
                      className="absolute top-1/2 right-2 -translate-y-1/2 rounded-md p-2
                               text-slate-400 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none"
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                      onClick={() => setShowPassword(!showPassword)}
                      disabled={isPending}
                    >
                      {showPassword ? (
                        <EyeOff className="size-4" aria-hidden="true" />
                      ) : (
                        <Eye className="size-4" aria-hidden="true" />
                      )}
                    </button>
                  </div>
                </div>

                {/* Submit button */}
                <div className="pt-2">
                  <Button
                    type="submit"
                    variant="accent"
                    className="w-full"
                    disabled={isPending}
                  >
                    <LogIn className="size-4" aria-hidden="true" />
                    <span>{isPending ? "Signing in..." : "Sign in"}</span>
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </main>
      </div>
    </div>
  );
}
