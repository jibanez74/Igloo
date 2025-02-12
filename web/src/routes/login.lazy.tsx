import { useState, useEffect } from "react";
import { createLazyFileRoute, useNavigate } from "@tanstack/react-router";
import useAuth from "@/hooks/useAuth";
import { FiUser, FiMail, FiLock } from "react-icons/fi";
import ErrorWarning from "@/components/ErrorWarning";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [isVisible, setIsVisible] = useState(false);

  const { setUser, user } = useAuth();

  const navigate = useNavigate();

  useEffect(() => {
    if (user) {
      navigate({ to: "/", replace: true });
    }
  }, [user, navigate]);

  useEffect(() => {
    if (error) {
      setIsVisible(true);

      const timer = setTimeout(() => {
        setIsVisible(false);
        setTimeout(() => setError(""), 300);
      }, 6000);

      return () => clearTimeout(timer);
    }
  }, [error]);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");
    setIsLoading(true);

    const formData = new FormData(e.currentTarget);
    const username = formData.get("username") as string;
    const email = formData.get("email") as string;
    const password = formData.get("password") as string;

    try {
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username,
          email,
          password,
        }),
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => null);

        throw new Error(
          errorData?.error || "Something went wrong. Please try again later."
        );
      }

      const data = await res.json();

      setUser(data.user);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("Something went wrong. Please try again later.");
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <main className='min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8'>
      <section className='max-w-md w-full space-y-8 bg-slate-900/50 backdrop-blur-sm p-8 rounded-xl shadow-lg shadow-blue-900/20'>
        <header>
          <h1 className='text-center text-3xl font-bold text-white'>
            Sign in to your account
          </h1>
        </header>
        <form
          className='mt-8 space-y-6'
          onSubmit={handleSubmit}
          aria-label='Login form'
        >
          <ErrorWarning error={error} isVisible={isVisible} />
          <fieldset className='space-y-4 rounded-md'>
            <legend className='sr-only'>Login credentials</legend>
            <div>
              <label htmlFor='username' className='sr-only'>
                Username
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiUser
                    className='h-5 w-5 text-blue-200'
                    aria-hidden='true'
                  />
                </div>
                <input
                  autoFocus
                  id='username'
                  name='username'
                  type='text'
                  required
                  className='appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400'
                  placeholder='Username'
                  aria-required='true'
                />
              </div>
            </div>
            <div>
              <label htmlFor='email' className='sr-only'>
                Email
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiMail
                    className='h-5 w-5 text-blue-200'
                    aria-hidden='true'
                  />
                </div>
                <input
                  id='email'
                  name='email'
                  type='email'
                  required
                  className='appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400'
                  placeholder='Email address'
                  aria-required='true'
                />
              </div>
            </div>
            <div>
              <label htmlFor='password' className='sr-only'>
                Password
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiLock
                    className='h-5 w-5 text-blue-200'
                    aria-hidden='true'
                  />
                </div>
                <input
                  id='password'
                  name='password'
                  type='password'
                  required
                  className='appearance-none relative block w-full px-10 py-2 border border-blue-900/20 bg-slate-800/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm placeholder:text-slate-400'
                  placeholder='Password'
                  aria-required='true'
                />
              </div>
            </div>
          </fieldset>

          <div>
            <button
              type='submit'
              disabled={isLoading}
              className='group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200'
              aria-busy={isLoading}
            >
              {isLoading ? "Signing in..." : "Sign in"}
            </button>
          </div>
        </form>
      </section>
    </main>
  );
}
