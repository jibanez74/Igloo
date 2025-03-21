import { Link } from "@tanstack/solid-router";
import { FiHome } from "solid-icons/fi";

export default function NotFound() {
  return (
    <section class="min-h-[calc(100vh-4rem)] flex items-center justify-center bg-blue-950">
      <div class="text-center px-4" aria-labelledby="not-found-title">
        <h1 class="text-9xl font-bold text-yellow-300/20">404</h1>

        <article class="mt-4">
          <h2
            id="not-found-title"
            class="text-3xl font-bold text-yellow-300 mb-2"
          >
            Page Not Found
          </h2>
          <p class="text-blue-200 mb-8">
            The page you're looking for doesn't exist or has been moved.
          </p>
          <nav>
            <Link
              to="/"
              class="inline-flex items-center gap-2 px-6 py-3 bg-yellow-300 text-blue-900 font-medium rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 transition-colors"
            >
              <FiHome class="w-5 h-5" aria-hidden="true" />
              Back to Home
            </Link>
          </nav>
        </article>
      </div>
    </section>
  );
}
