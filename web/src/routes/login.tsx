import { useState, useSyncExternalStore } from "react";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { showSuccess, showError } from "@/lib/toast-helpers";
import { Snowflake, Mail, Lock, Eye, EyeOff, LogIn } from "lucide-react";
import { login } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import { loginSearchSchema } from "@/lib/route-search";
import { getActiveTheme, subscribeTheme } from "@/lib/theme";
import loginBgDark from "@/assets/images/login-bg-dark.webp";
import loginBgLight from "@/assets/images/login-bg-light.webp";
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
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  MOTION_PAGE_ENTER_CLASS,
  USER_PASSWORD_MAX_LENGTH,
  USER_PASSWORD_MIN_LENGTH,
} from "@/lib/constants";
import {
  inputIconClassName,
  lightInputActionClassName,
  lightInputClassName,
} from "@/lib/input-styles";
import { cn } from "@/lib/utils";

const pageTitle = "Sign In - Igloo";
const pageDescription = "Sign in to access your personal Igloo media library.";

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
  component: LoginPage,
});

function LoginPage() {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const navigate = Route.useNavigate();
  const { redirect: redirectTo } = Route.useSearch();
  const { queryClient } = Route.useRouteContext();
  // Pick the backdrop to match the active theme (tracks the `.dark` class the
  // anti-flash script sets pre-paint, so the first render is already correct).
  const theme = useSyncExternalStore(subscribeTheme, getActiveTheme);
  const loginBg = theme === "light" ? loginBgLight : loginBgDark;

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

      queryClient.removeQueries();
      const authRes = await queryClient.fetchQuery(
        authUserQueryOpts({ revalidate: true }),
      );
      if (authRes.error) {
        showError(
          "Login failed",
          authRes.message || "Unable to establish the authenticated session",
        );
        setIsSubmitting(false);
        return;
      }

      showSuccess("Welcome back!", res.message || "Login successful");

      await navigate({
        to: redirectTo,
        replace: true,
      });
    } catch (err) {
      console.debug("login post-auth flow failed", err);
      showError(
        "Login failed",
        "Something went wrong after sign-in. Please try again.",
      );
      // This line is the catch-block reset. After a successful login the form stays
      // disabled on purpose while the router navigates away.
      // react-doctor-disable-next-line react-doctor/no-loading-flag-reset-outside-finally
      setIsSubmitting(false);
    }
  };

  return (
    <main className="relative flex min-h-svh items-center justify-center px-4">
      {/* React 19 Document Metadata */}
      <title>{pageTitle}</title>
      <meta name="description" content={pageDescription} />

      <img
        src={loginBg}
        alt=""
        aria-hidden="true"
        className="absolute inset-0 size-full object-cover"
        decoding="async"
        fetchPriority="high"
      />
      {/*
        Photographic scrim. Dark mode keeps its bottom-weighted darkening scrim;
        light mode uses a cool "frost" lift so the bright photo stays vivid while
        still giving the card enough contrast (a flat `bg-background/70` inverted
        to a milky wash in light mode).
      */}
      <div
        className={cn(
          "absolute inset-0 bg-linear-to-b",
          "from-white/25 via-slate-200/35 to-slate-400/60",
          "dark:from-background/40 dark:via-background/65 dark:to-background/85",
        )}
      />

      <Card
        className={cn(
          MOTION_PAGE_ENTER_CLASS,
          "z-10 w-full max-w-md bg-card/85 shadow-2xl backdrop-blur-sm",
          "border-white/60 dark:border-border",
        )}
      >
        <CardHeader className="pb-2 text-center">
          <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-muted">
            <Snowflake className="size-5 text-primary" aria-hidden="true" />
          </div>
          <CardTitle asChild className="text-2xl font-semibold tracking-tight text-foreground">
            <h1>Welcome to Igloo</h1>
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
                <Mail className={inputIconClassName} aria-hidden="true" />
                <Input
                  autoFocus
                  type="email"
                  id="email"
                  name="email"
                  inputMode="email"
                  autoComplete="username"
                  required
                  className={cn("pl-10", lightInputClassName)}
                  disabled={isSubmitting}
                />
              </div>
            </div>

            {/* Password field */}
            <div className="space-y-1">
              <Label htmlFor="password">Password</Label>
              <div className="relative">
                <Lock className={inputIconClassName} aria-hidden="true" />
                <Input
                  type={showPassword ? "text" : "password"}
                  minLength={USER_PASSWORD_MIN_LENGTH}
                  maxLength={USER_PASSWORD_MAX_LENGTH}
                  id="password"
                  name="password"
                  autoComplete="current-password"
                  required
                  className={cn("px-10", lightInputClassName)}
                  disabled={isSubmitting}
                />
                <button
                  type="button"
                  className={cn(
                    "absolute top-1/2 right-2 -translate-y-1/2 rounded-md p-2",
                    FOCUS_VISIBLE_RING_CLASS,
                    MOTION_MICRO_COLORS_CLASS,
                    lightInputActionClassName,
                  )}
                  aria-label={showPassword ? "Hide password" : "Show password"}
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
  );
}
