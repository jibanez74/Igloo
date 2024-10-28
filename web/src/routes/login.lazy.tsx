import { useState } from "react";
import { useNavigate, createLazyFileRoute } from "@tanstack/react-router";
import { FaEnvelope, FaLock, FaSignInAlt, FaUser } from "react-icons/fa";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import { useAppContext } from "@/AppContext";
import type { User } from "@/types/User";
import type { Res } from "@/types/Response";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const navigate = useNavigate();

  const { setUser } = useAppContext();

  const handleLogin = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    if (error) setError("");

    const form = e.currentTarget;
    const formData = new FormData(form);

    const authData = {
      email: formData.get("email") as string,
      username: formData.get("username"),
      password: formData.get("password") as string,
    };

    try {
      const res = await fetch("/api/v1/login", {
        method: "post",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(authData),
      });

      const r: Res<User> = await res.json();

      if (r.error) {
        setError(`${res.status} - ${r.message}`);
        return;
      }

      if (r.data) {
        setUser(r.data);

        alert(JSON.stringify(r.data));

        navigate({
          to: "/",
          replace: true,
        });
      }
    } catch (err: unknown) {
      console.error(err);
      setError("unable to process your request");
      setLoading(false);
    }
  };

  return (
    <section className='flex items-center justify-center min-h-screen'>
      <div className='w-full max-w-md'>
        <form
          onSubmit={handleLogin}
          className='bg-primary shadow-md rounded px-8 pt-6 pb-8 mb-4'
          aria-labelledby='login-heading'
        >
          <h2
            id='login-heading'
            className='text-2xl font-bold mb-6 text-light text-center'
          >
            Login
          </h2>

          <div aria-live='polite' aria-atomic='true'>
            {error && <Alert msg={error} />}
          </div>

          <div className='mb-4'>
            <label
              className='block text-light text-sm font-bold mb-2'
              htmlFor='email'
            >
              Email
            </label>
            <div className='relative'>
              <span className='absolute inset-y-0 left-0 flex items-center pl-3'>
                <FaEnvelope className='text-gray-400' />
              </span>
              <input
                autoFocus
                type='email'
                className='shadow appearance-none border rounded w-full py-2 pl-10 pr-3 text-dark leading-tight focus:outline-none focus:shadow-outline'
                id='email'
                name='email'
                required
                disabled={loading}
                aria-invalid={!!error}
                aria-describedby={error ? "login-error" : undefined}
              />
            </div>
          </div>

          <div className='mb-4'>
            <label
              className='block text-light text-sm font-bold mb-2'
              htmlFor='username'
            >
              Username
            </label>
            <div className='relative'>
              <span className='absolute inset-y-0 left-0 flex items-center pl-3'>
                <FaUser className='text-gray-400' />
              </span>
              <input
                type='text'
                className='shadow appearance-none border rounded w-full py-2 pl-10 pr-3 text-dark leading-tight focus:outline-none focus:shadow-outline'
                id='username'
                name='username'
                required
                disabled={loading}
                aria-invalid={!!error}
                aria-describedby={error ? "login-error" : undefined}
              />
            </div>
          </div>

          <div className='mb-6'>
            <label
              className='block text-light text-sm font-bold mb-2'
              htmlFor='password'
            >
              Password
            </label>
            <div className='relative'>
              <span className='absolute inset-y-0 left-0 flex items-center pl-3'>
                <FaLock className='text-gray-400' />
              </span>
              <input
                type='password'
                className='shadow appearance-none border rounded w-full py-2 pl-10 pr-3 text-dark mb-3 leading-tight focus:outline-none focus:shadow-outline'
                id='password'
                name='password'
                minLength={9}
                maxLength={128}
                required
                disabled={loading}
                aria-invalid={!!error}
                aria-describedby={error ? "login-error" : undefined}
              />
            </div>
          </div>
          <div className='flex items-center justify-between'>
            <button
              className='bg-secondary hover:bg-success text-dark font-bold py-2 px-4 rounded focus:outline-none focus:shadow-outline transition duration-300 flex items-center'
              type='submit'
              disabled={loading}
              aria-label='Sign in to your account'
            >
              {loading ? (
                <Spinner />
              ) : (
                <>
                  <FaSignInAlt className='mr-2' />
                  Sign In
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </section>
  );
}
