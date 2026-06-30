import { useState } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import { showSuccess, showError } from "@/lib/toast-helpers";
import { Snowflake, Mail, Lock, Eye, EyeOff, LogIn } from "lucide-react";
import { login } from "@/lib/api";
import { authUserGuardQueryOpts } from "@/lib/query-opts";
import loginBg from "@/assets/images/login-bg.webp";
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
import { MOTION_PAGE_ENTER_CLASS } from "@/lib/constants";
import { inputIconClassName, lightInputClassName } from "@/lib/input-styles";
import { cn } from "@/lib/utils";

const pageTitle = "Sign In - Igloo";
const pageDescription = "Sign in to access your personal Igloo media library.";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const navigate = Route.useNavigate();
  const { redirect } = Route.useSearch();
  const { queryClient } = Route.useRouteContext();

  const loginHandler = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsSubmitting(true);
    const formData = new FormData(e.currentTarget);
    const email = formData.get("email") as string;
    const password = formData.get("password") as string;

    try {
      const res = await login(email, password);

      if (res.error) {
        showError(
          "Login failed",
          res.message || "An error occurred during login"
        );
        setIsSubmitting(false);
        return;
      }

      showSuccess("Welcome back!", res.message || "Login successful");

      queryClient.removeQueries();
      await queryClient.fetchQuery(authUserGuardQueryOpts());

      await navigate({
        to: redirect,
        replace: true,
      });
    } catch (err) {
      console.error(err);
      showError(
        "Login failed",
        "Something went wrong after sign-in. Please try again.",
      );
      setIsSubmitting(false);
    }
  };

  return (
    <div className="h-full bg-background text-foreground antialiased">
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <div className="relative min-h-screen">
        <img
          src={loginBg}
          alt=""
          aria-hidden="true"
          className="absolute inset-0 size-full object-cover"
          decoding="async"
          fetchPriority="high"
        />
        {/* Dark overlay */}
        <div className="absolute inset-0 bg-background/70" />

        <main className="relative z-10 flex min-h-screen items-center justify-center px-4">
          <Card
            className={cn(
              MOTION_PAGE_ENTER_CLASS,
              "w-full max-w-md border-border bg-card/80 shadow-xl backdrop-blur-sm",
            )}
          >
            <CardHeader className="pb-2 text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-muted">
                <Snowflake
                  className="size-5 text-primary"
                  aria-hidden="true"
                />
              </div>
              <CardTitle className="text-2xl font-semibold tracking-tight text-foreground">
                Welcome to Igloo
              </CardTitle>
              <CardDescription className="text-muted-foreground">
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
                      className={inputIconClassName}
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
                      className={`pl-10 ${lightInputClassName}`}
                      disabled={isSubmitting}
                    />
                  </div>
                </div>

                {/* Password field */}
                <div className="space-y-1">
                  <Label htmlFor="password">Password</Label>
                  <div className="relative">
                    <Lock
                      className={inputIconClassName}
                      aria-hidden="true"
                    />
                    <Input
                      type={showPassword ? "text" : "password"}
                      minLength={9}
                      maxLength={128}
                      id="password"
                      name="password"
                      autoComplete="current-password"
                      required
                      className={`px-10 ${lightInputClassName}`}
                      disabled={isSubmitting}
                    />
                    <button
                      type="button"
                      className="absolute top-1/2 right-2 -translate-y-1/2 rounded-md p-2
                               text-muted-foreground hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-none"
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                      onClick={() => setShowPassword(!showPassword)}
                      disabled={isSubmitting}
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
                    disabled={isSubmitting}
                  >
                    <LogIn className="size-4" aria-hidden="true" />
                    <span>{isSubmitting ? "Signing in..." : "Sign in"}</span>
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
