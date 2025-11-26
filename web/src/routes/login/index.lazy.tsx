import { useState, useTransition } from "react";
import { createLazyFileRoute } from "@tanstack/react-router";
import loginBg from "@/assets/images/login-bg.jpg";

export const Route = createLazyFileRoute("/login/")({
  component: LoginPage,
});

function LoginPage() {
  const [isPending, setIsPending] = useTransition();
  const [showPassword, setShowPassword] = useState(false);

  const loginHandler = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
  };

  return (
    <div className='h-full bg-slate-900 text-slate-100 antialiased'>
      <div className='relative min-h-screen'>
        <div
          className='absolute inset-0 bg-cover bg-center bg-no-repeat'
          style={{ backgroundImage: `url(${loginBg})` }}
        ></div>
        <div className='absolute inset-0 bg-slate-950/70'></div>

        <main className='relative z-10 min-h-screen flex items-center justify-center px-4'>
          <section
            id='login-card'
            className='w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/80 backdrop-blur shadow-xl p-6 sm:p-8
               opacity-0 translate-y-2 transition-transform duration-500 ease-out will-change-transform'
          >
            <div className='mb-6 text-center'>
              <div className='mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-slate-800'>
                <i
                  className='fa-solid fa-igloo text-amber-400 text-xl'
                  aria-hidden='true'
                ></i>
              </div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                Welcome to Igloo
              </h1>
              <p className='mt-1 text-slate-400 text-sm'>
                Sign in to access your private media library.
              </p>
            </div>

            <form onSubmit={loginHandler} id='login-form' className='space-y-4'>
              <div>
                <label htmlFor='email' className='block text-sm mb-1'>
                  Email
                </label>
                <div className='relative'>
                  <i
                    className='fa-regular fa-envelope absolute left-3 top-1/2 -translate-y-1/2 text-slate-400'
                    aria-hidden='true'
                  ></i>
                  <input
                    autoFocus
                    type='email'
                    id='email'
                    name='email'
                    inputMode='email'
                    autoComplete='username'
                    required
                    className='w-full rounded-lg bg-slate-800/80 pl-10 pr-3 py-2 text-sm
                       focus:outline-none focus:ring-2 focus:ring-amber-400'
                  disabled={isPending}
                  />
                </div>
              </div>

              <div>
                <label htmlFor='password' className='block text-sm mb-1'>
                  Password
                </label>
                <div className='relative'>
                  <i
                    className='fa-solid fa-lock absolute left-3 top-1/2 -translate-y-1/2 text-slate-400'
                    aria-hidden='true'
                  ></i>
                  <input
                    type={showPassword ? "text" : "password"}
                    minLength={9}
                    maxLength={128}
                    id='password'
                    name='password'
                    required
                    className='w-full rounded-lg bg-slate-800/80 pl-10 pr-10 py-2 text-sm
                       focus:outline-none focus:ring-2 focus:ring-amber-400'
                       disabled={isPending}
                  />

                  <button
                    type='button'
                    id='togglePassword'
                    className='absolute right-2 top-1/2 -translate-y-1/2 p-2 rounded-md
                             text-slate-400 hover:text-white focus:outline-none focus:ring-2 focus:ring-amber-400'
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
                    ></i>
                  </button>
                </div>
              </div>

              <div className='pt-2'>
                <button
                  type='submit'
                  className='w-full inline-flex items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2
                     text-sm font-semibold text-white hover:bg-blue-500
                     focus:outline-none focus:ring-2 focus:ring-amber-400'
                     disabled={isPending}
                >
                  <i
                    className='fa-solid fa-right-to-bracket'
                    aria-hidden='true'
                  ></i>
                  <span>Sign in</span>
                </button>
              </div>
            </form>
          </section>
        </main>
      </div>
    </div>
  );
}
