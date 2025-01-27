import { useState } from "react";
import { FiUser, FiLock } from "react-icons/fi";
import useAuth from "@/hooks/useAuth";

export default function Login() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const { setUser } = useAuth();

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    const formData = new FormData(e.currentTarget);
    const username = formData.get("username") as string;
    const password = formData.get("password") as string;

    try {
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        throw new Error("Invalid credentials");
      }

      const data = await response.json();
      setUser(data.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className='min-h-screen flex items-center justify-center bg-gradient-to-br from-sky-950 via-blue-900 to-indigo-950'>
      <div className='max-w-md w-full space-y-8 p-10 bg-slate-900/50 backdrop-blur-sm rounded-xl shadow-xl'>
        <div className='text-center'>
          <h2 className='mt-6 text-3xl font-bold text-white'>Welcome back</h2>
          <p className='mt-2 text-sm text-blue-200'>Sign in to your account</p>
        </div>
        <form className='mt-8 space-y-6' onSubmit={handleSubmit}>
          <div className='rounded-md shadow-sm -space-y-px'>
            <div className='relative'>
              <FiUser className='absolute top-3 left-3 text-blue-300' />
              <input
                id='username'
                name='username'
                type='text'
                required
                className='appearance-none rounded-t-md relative block w-full px-10 py-2 border border-slate-700 bg-slate-800/50 placeholder-blue-300 text-white focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10'
                placeholder='Username'
              />
            </div>
            <div className='relative'>
              <FiLock className='absolute top-3 left-3 text-blue-300' />
              <input
                id='password'
                name='password'
                type='password'
                required
                className='appearance-none rounded-b-md relative block w-full px-10 py-2 border border-slate-700 bg-slate-800/50 placeholder-blue-300 text-white focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10'
                placeholder='Password'
              />
            </div>
          </div>

          {error && (
            <div className='text-red-400 text-sm text-center'>{error}</div>
          )}

          <div>
            <button
              type='submit'
              disabled={isLoading}
              className='group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed'
            >
              {isLoading ? "Signing in..." : "Sign in"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
