import { useState, useTransition, useEffect, useRef } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import { toast } from "sonner";
import { login } from "@/lib/api";
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

export const Route = createLazyFileRoute("/login/")({
  component: LoginPage,
});

function LoginPage() {
  const [isPending, startTransition] = useTransition();
  const [showPassword, setShowPassword] = useState(false);
  const cardRef = useRef<HTMLDivElement>(null);
  const navigate = Route.useNavigate();
  const { redirect } = Route.useSearch();
  const { queryClient } = Route.useRouteContext();

  // Entrance animation
  useEffect(() => {
    const card = cardRef.current;
    if (!card) return;

    // Check for reduced motion preference
    const prefersReduced = window.matchMedia(
      "(prefers-reduced-motion: reduce)"
    ).matches;

    if (prefersReduced) {
      card.classList.remove("opacity-0", "translate-y-2");
      return;
    }

    // Trigger animation on next frame
    requestAnimationFrame(() => {
      card.classList.remove("opacity-0", "translate-y-2");
    });
  }, []);

  const loginHandler = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    startTransition(async () => {
      const formData = new FormData(e.currentTarget);
      const email = formData.get("email") as string;
      const password = formData.get("password") as string;

      const res = await login(email, password);

      if (res.error) {
        toast.error("Error", {
          description: res.message || "An error occurred during login",
        });
      } else {
        toast.success("Success", {
          description: res.message || "Login successful",
        });

        await queryClient.invalidateQueries();

        navigate({
          to: redirect,
          from: "/login",
          replace: true,
        });
      }
    });
  };

  return (
    <div className='h-full bg-slate-900 text-slate-100 antialiased'>
      <div className='relative min-h-screen'>
        {/* Background image */}
        <div
          className='absolute inset-0 bg-cover bg-center bg-no-repeat'
          style={{ backgroundImage: `url(${loginBg})` }}
        />
        {/* Dark overlay */}
        <div className='absolute inset-0 bg-slate-950/70' />

        <main className='relative z-10 flex min-h-screen items-center justify-center px-4'>
          <Card
            ref={cardRef}
            className='w-full max-w-md translate-y-2 border-slate-800 bg-slate-900/80 opacity-0
               shadow-xl backdrop-blur-sm transition-all duration-500 ease-out will-change-transform'
          >
            <CardHeader className='pb-2 text-center'>
              <div className='mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-slate-800'>
                <i
                  className='fa-solid fa-igloo text-xl text-amber-400'
                  aria-hidden='true'
                />
              </div>
              <CardTitle className='text-2xl font-semibold tracking-tight text-slate-100'>
                Welcome to Igloo
              </CardTitle>
              <CardDescription className='text-slate-400'>
                Sign in to access your private media library.
              </CardDescription>
            </CardHeader>

            <CardContent>
              <form onSubmit={loginHandler} className='space-y-4'>
                {/* Email field */}
                <div className='space-y-1'>
                  <Label htmlFor='email'>Email</Label>
                  <div className='relative'>
                    <i
                      className='fa-regular fa-envelope absolute top-1/2 left-3 z-10 -translate-y-1/2 text-slate-400'
                      aria-hidden='true'
                    />
                    <Input
                      autoFocus
                      type='email'
                      id='email'
                      name='email'
                      inputMode='email'
                      autoComplete='username'
                      required
                      className='pl-10'
                      disabled={isPending}
                    />
                  </div>
                </div>

                {/* Password field */}
                <div className='space-y-1'>
                  <Label htmlFor='password'>Password</Label>
                  <div className='relative'>
                    <i
                      className='fa-solid fa-lock absolute top-1/2 left-3 z-10 -translate-y-1/2 text-slate-400'
                      aria-hidden='true'
                    />
                    <Input
                      type={showPassword ? "text" : "password"}
                      minLength={9}
                      maxLength={128}
                      id='password'
                      name='password'
                      required
                      className='px-10'
                      disabled={isPending}
                    />
                    <button
                      type='button'
                      className='absolute top-1/2 right-2 -translate-y-1/2 rounded-md p-2
                               text-slate-400 hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none'
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                      onClick={() => setShowPassword(!showPassword)}
                      disabled={isPending}
                    >
                      <i
                        className={
                          showPassword
                            ? "fa-regular fa-eye-slash"
                            : "fa-regular fa-eye"
                        }
                        aria-hidden='true'
                      />
                    </button>
                  </div>
                </div>

                {/* Submit button */}
                <div className='pt-2'>
                  <Button
                    type='submit'
                    className='w-full bg-amber-500 font-semibold text-slate-900 hover:bg-amber-400'
                    disabled={isPending}
                  >
                    <i
                      className='fa-solid fa-right-to-bracket'
                      aria-hidden='true'
                    />
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
