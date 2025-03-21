import { createSignal } from "solid-js";
import { createLazyFileRoute } from "@tanstack/solid-router";
import { FiUser, FiMail, FiLock } from "solid-icons/fi";
import ErrorWarning from "../components/ErrorWarning";
import { setAuthState } from "../stores/authStore";

export const Route = createLazyFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const [ username, setUsername] = createSignal("");
  const [email, setEmail] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [error, setError] = createSignal("");
  const [isLoading, setIsLoading] = createSignal(false);
  const [isVisible, setIsVisible] = createSignal(false);

  const navigate = Route.useNavigate();

  const handleSubmit = async (e: Event) => {
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
        credentials: "same-origin",
        body: JSON.stringify({
          username: username(),
          email: email(),
          password: password(),
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || res.statusText);
        setIsVisible(true);
        return;
      }

      setAuthState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });

      navigate({
        to: "/",
        from: "/login",
        replace: true,
      });
    } catch (err) {
      console.error(err);
      setError("A network error occurred");
      setIsVisible(true);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center px-4 sm:px-6 lg:px-8">
      <section class="max-w-md w-full space-y-8 bg-blue-950/50 backdrop-blur-sm p-8 rounded-xl shadow-lg shadow-blue-900/20">
        <header>
          <h1 class="text-center text-3xl font-bold text-white">
            Sign in to your account
          </h1>
        </header>

        <form
          class="mt-8 space-y-6"
          onSubmit={handleSubmit}
          aria-label="Login form"
        >
          <ErrorWarning error={error()} isVisible={isVisible()} />
          <fieldset class="space-y-4 rounded-md">
            <legend class="sr-only">Login credentials</legend>

            <div>
              <label for="username" class="sr-only">
                Username
              </label>

              <div class="relative">
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiUser class="h-5 w-5 text-yellow-300" aria-hidden="true" />
                </div>

                <input
                  autofocus
                  id="username"
                  name="username"
                  type="text"
                  required
                  class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"
                  placeholder="Username"
                  aria-required="true"
                  value={username()}
                  onInput={(e) => setUsername(e.currentTarget.value)}
                />
              </div>
            </div>

            <div>
              <label for="email" class="sr-only">
                Email
              </label>

              <div class="relative">
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiMail class="h-5 w-5 text-yellow-300" aria-hidden="true" />
                </div>

                <input
                  id="email"
                  name="email"
                  type="email"
                  required
                  class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"
                  placeholder="Email address"
                  aria-required="true"
                  value={email()}
                  onInput={(e) => setEmail(e.currentTarget.value)}
                />
              </div>
            </div>

            <div>
              <label for="password" class="sr-only">
                Password
              </label>

              <div class="relative">
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiLock class="h-5 w-5 text-yellow-300" aria-hidden="true" />
                </div>

                <input
                  id="password"
                  name="password"
                  type="password"
                  required
                  class="appearance-none relative block w-full px-10 py-2 border border-blue-800/20 bg-blue-900/50 text-white rounded-md focus:outline-none focus:ring-2 focus:ring-yellow-300 focus:border-yellow-300 focus:z-10 sm:text-sm placeholder:text-blue-200/50"
                  placeholder="Password"
                  aria-required="true"
                  value={password()}
                  onInput={(e) => setPassword(e.currentTarget.value)}
                />
              </div>
            </div>
          </fieldset>

          <div>
            <button
              type="submit"
              disabled={isLoading()}
              class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200"
              aria-busy={isLoading()}
            >
              {isLoading() ? "Signing in..." : "Sign in"}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
