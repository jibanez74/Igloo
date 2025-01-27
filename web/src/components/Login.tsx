import { useState } from "react";
import useAuth from "../hooks/useAuth";
import { FiUser, FiMail, FiLock, FiLogIn, FiLoader } from "react-icons/fi";
import iglooLogo from "../assets/images/igloo.png";

export default function Login() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { user, setUser } = useAuth();

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      const formData = new FormData(e.currentTarget);
      const response = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username: formData.get("username"),
          email: formData.get("email"),
          password: formData.get("password"),
        }),
        credentials: "include", // This is important for cookies
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.message || "Failed to login");
      }

      const data = await response.json();

      setUser(data.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setIsLoading(false);
    }
  };

  if (user) {
    alert("User is logged in");
  }

  return (
    <main className='min-h-screen flex items-center justify-center bg-gradient-to-br from-sky-950 via-blue-900 to-indigo-950'>
      <section
        className='bg-white/10 p-8 rounded-lg w-full max-w-md backdrop-blur-sm'
        aria-label='Login form'
      >
        <header className='flex items-center justify-center gap-4 mb-8'>
          <img src={iglooLogo} alt='Igloo' className='h-12 w-auto' />
          <h1 className='text-4xl font-bold text-white'>Sign In</h1>
        </header>
        <div className='text-center mb-8'>
          <h2 className='text-2xl font-medium text-white mb-2'>
            Welcome Back!
          </h2>
          <p className='text-blue-200'>
            Enter your credentials to access your media
          </p>
        </div>
        {error && (
          <div className='p-4 mb-6 bg-red-500/10 border border-red-500 rounded text-red-500 text-center'>
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className='space-y-6'>
          <div className='relative'>
            <label htmlFor='username' className='sr-only'>
              Username
            </label>
            <FiUser
              className='absolute left-4 top-1/2 -translate-y-1/2 text-blue-200'
              aria-hidden='true'
            />
            <input
              autoFocus
              type='text'
              className='w-full p-4 pl-12 rounded bg-blue-950/50 border border-blue-800 text-white placeholder-blue-300 focus:outline-none focus:border-yellow-400 focus:ring-1 focus:ring-yellow-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
              id='username'
              name='username'
              required
              maxLength={20}
              placeholder='Username'
              disabled={isLoading}
            />
          </div>
          <div className='relative'>
            <label htmlFor='email' className='sr-only'>
              Email
            </label>
            <FiMail
              className='absolute left-4 top-1/2 -translate-y-1/2 text-blue-200'
              aria-hidden='true'
            />
            <input
              type='email'
              className='w-full p-4 pl-12 rounded bg-blue-950/50 border border-blue-800 text-white placeholder-blue-300 focus:outline-none focus:border-yellow-400 focus:ring-1 focus:ring-yellow-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
              id='email'
              name='email'
              required
              placeholder='email'
              disabled={isLoading}
            />
          </div>
          <div className='relative'>
            <label htmlFor='password' className='sr-only'>
              Password
            </label>
            <FiLock
              className='absolute left-4 top-1/2 -translate-y-1/2 text-blue-200'
              aria-hidden='true'
            />
            <input
              type='password'
              className='w-full p-4 pl-12 rounded bg-blue-950/50 border border-blue-800 text-white placeholder-blue-300 focus:outline-none focus:border-yellow-400 focus:ring-1 focus:ring-yellow-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
              id='password'
              name='password'
              required
              minLength={9}
              maxLength={128}
              placeholder='Password'
              disabled={isLoading}
            />
          </div>
          <button
            type='submit'
            disabled={isLoading}
            className='w-full py-4 bg-blue-600 text-white rounded font-semibold hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-yellow-400 focus:ring-offset-2 focus:ring-offset-blue-950 transition-all duration-300 ease-in-out flex items-center justify-center gap-2 group disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-blue-600'
          >
            <span>{isLoading ? "Signing In..." : "Sign In"}</span>
            {isLoading ? (
              <FiLoader className='w-5 h-5 animate-spin' aria-hidden='true' />
            ) : (
              <FiLogIn
                className='w-5 h-5 transform transition-transform duration-300 ease-in-out group-hover:translate-x-1'
                aria-hidden='true'
              />
            )}
          </button>
        </form>
      </section>
    </main>
  );
}
