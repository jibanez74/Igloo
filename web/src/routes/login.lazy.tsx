import { useState } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import { FiUser, FiMail, FiLock } from "react-icons/fi";
import ErrorWarning from "../components/ErrorWarning";
import useAuth from "../hooks/useAuth";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isVisible, setIsVisible] = useState(false);
  const { login } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");
    setIsVisible(false);

    try {
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username,
          email,
          password,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || res.statusText);
        setIsVisible(true);
        return;
      }

      login(data);
    } catch (err) {
      console.error(err);
      setError("A network error occurred");
      setIsVisible(true);
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
                  value={username}
                  onChange={e => setUsername(e.target.value)}
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
                  value={email}
                  onChange={e => setEmail(e.target.value)}
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
                  value={password}
                  onChange={e => setPassword(e.target.value)}
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
