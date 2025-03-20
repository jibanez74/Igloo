import { createSignal, onCleanup, onMount, Show } from "solid-js";
import { Link, useNavigate } from "@tanstack/solid-router";
import {
  FiHome,
  FiFilm,
  FiTv,
  FiMusic,
  FiMenu,
  FiX,
  FiSettings,
  FiLogOut,
  FiLogIn,
} from "solid-icons/fi";
import iglooLogo from "../assets/images/logo-alt.png";
import { authState, setAuthState } from "../stores/authStore";

export default function Navbar() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = createSignal(false);
  let mobileMenuRef: HTMLDivElement | undefined;
  let menuButtonRef: HTMLButtonElement | undefined;
  const { isAuthenticated } = authState;

  const navigate = useNavigate();

  onMount(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        isMobileMenuOpen() &&
        mobileMenuRef &&
        !mobileMenuRef.contains(event.target as Node) &&
        menuButtonRef &&
        !menuButtonRef.contains(event.target as Node)
      ) {
        setIsMobileMenuOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);

    onCleanup(() => {
      document.removeEventListener("mousedown", handleClickOutside);
    });
  });

  const toggleMobileMenu = () => setIsMobileMenuOpen(!isMobileMenuOpen());

  const handleSignOut = async () => {
    try {
      await fetch("/api/v1/auth/logout", {
        method: "post",
        credentials: "include",
      });

      setAuthState({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });

      navigate({
        to: "/login",
        replace: true,
      });
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <>
      {/* Fade Overlay */}
      <div
        class={`fixed inset-0 bg-gradient-to-b from-blue-950/80 to-blue-900/80 backdrop-blur-sm transition-all duration-300 md:hidden ${
          isMobileMenuOpen() ? "opacity-100" : "opacity-0 pointer-events-none"
        }`}
        onClick={() => setIsMobileMenuOpen(false)}
        aria-hidden="true"
      />

      <nav class="fixed top-0 left-0 right-0 bg-blue-950/95 shadow-lg shadow-blue-900/20 backdrop-blur-sm z-50">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div class="flex items-center justify-between h-16">
            {/* Left side - Logo and Navigation */}
            <div class="flex items-center gap-8">
              <Link
                to="/"
                class="flex items-center gap-2 text-white hover:text-yellow-300 transition-colors"
                preload="intent"
              >
                <img src={iglooLogo} alt="Igloo" class="h-8 w-auto" />
                <span class="text-xl font-semibold bg-gradient-to-r from-yellow-300 to-yellow-200 text-transparent bg-clip-text">
                  Igloo
                </span>
              </Link>

              {/* Desktop Navigation Links */}
              <div class="hidden md:flex items-center gap-1">
                {/* Home link - always visible */}
                <Link
                  to="/"
                  preload="intent"
                  class="px-3 py-2 rounded-md text-sm font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2 relative group"
                >
                  <FiHome class="w-4 h-4" aria-hidden={true} />
                  Home
                  <span class="absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-yellow-300 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
                </Link>

                {/* Authenticated Navigation Links */}
                <Show when={isAuthenticated}>
                  <Link
                    to="/movies"
                    preload="intent"
                    class="px-3 py-2 rounded-md text-sm font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2 relative group"
                  >
                    <FiFilm class="w-4 h-4" aria-hidden={true} />
                    Movies
                    <span class="absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-yellow-300 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
                  </Link>

                  <Link
                    to="/tvshows"
                    preload="intent"
                    class="px-3 py-2 rounded-md text-sm font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2 relative group"
                  >
                    <FiTv class="w-4 h-4" aria-hidden={true} />
                    TV Shows
                    <span class="absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-yellow-300 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
                  </Link>

                  <Link
                    to="/music"
                    preload="intent"
                    class="px-3 py-2 rounded-md text-sm font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2 relative group"
                  >
                    <FiMusic class="w-4 h-4" aria-hidden={true} />
                    Music
                    <span class="absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-yellow-300 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
                  </Link>
                </Show>
              </div>
            </div>

            {/* Right side - Settings, Sign In/Out, and Mobile Menu Button */}
            <div class="flex items-center gap-2">
              <Show
                when={isAuthenticated}
                fallback={
                  <>
                    <Link
                      to="/login"
                      class="text-white hover:text-yellow-300 px-3 py-2 rounded-md text-sm font-medium hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2"
                    >
                      <FiLogIn aria-hidden={true} /> Sign In
                    </Link>

                    {/* Mobile Menu Button for non-authenticated users */}
                    <button
                      ref={menuButtonRef}
                      type="button"
                      class="md:hidden p-2 rounded-md text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out"
                      onClick={toggleMobileMenu}
                      aria-expanded={isMobileMenuOpen()}
                      aria-label="Toggle mobile menu"
                    >
                      <Show
                        when={isMobileMenuOpen()}
                        fallback={<FiMenu class="w-6 h-6" aria-hidden={true} />}
                      >
                        <FiX class="w-6 h-6" aria-hidden={true} />
                      </Show>
                    </button>
                  </>
                }
              >
                <>
                  {/* Settings Link */}
                  <Link
                    to="/"
                    class="text-white hover:text-yellow-300 px-3 py-2 rounded-md text-sm font-medium hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2"
                  >
                    <FiSettings aria-hidden={true} /> Settings
                  </Link>

                  {/* Sign Out Button */}
                  <button
                    onClick={handleSignOut}
                    class="text-white hover:text-yellow-300 px-3 py-2 rounded-md text-sm font-medium hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out flex items-center gap-2"
                  >
                    <FiLogOut aria-hidden={true} /> Sign Out
                  </button>

                  {/* Mobile Menu Button for authenticated users */}
                  <button
                    ref={menuButtonRef}
                    type="button"
                    class="md:hidden p-2 rounded-md text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-opacity duration-300 ease-in-out"
                    onClick={toggleMobileMenu}
                    aria-expanded={isMobileMenuOpen()}
                    aria-label="Toggle mobile menu"
                  >
                    <Show
                      when={isMobileMenuOpen()}
                      fallback={<FiMenu class="w-6 h-6" aria-hidden={true} />}
                    >
                      <FiX class="w-6 h-6" aria-hidden={true} />
                    </Show>
                  </button>
                </>
              </Show>
            </div>
          </div>
        </div>

        {/* Mobile Navigation Menu */}
        <div
          ref={mobileMenuRef}
          class={`md:hidden transition-all duration-300 ease-in-out ${
            isMobileMenuOpen()
              ? "max-h-96 opacity-100"
              : "max-h-0 opacity-0 pointer-events-none"
          }`}
        >
          <div class="px-2 pt-2 pb-3 space-y-1 bg-blue-950/95 backdrop-blur-sm border-t border-blue-800/20">
            {/* Home link - always visible */}
            <Link
              to="/"
              preload="intent"
              class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
              onClick={() => setIsMobileMenuOpen(false)}
            >
              <FiHome class="w-5 h-5" aria-hidden={true} />
              Home
            </Link>

            {/* Authenticated Navigation Links */}
            <Show when={isAuthenticated}>
              <Link
                to="/movies"
                preload="intent"
                class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <FiFilm class="w-5 h-5" aria-hidden={true} />
                Movies
              </Link>

              <Link
                to="/tvshows"
                preload="intent"
                class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <FiTv class="w-5 h-5" aria-hidden={true} />
                TV Shows
              </Link>

              <Link
                to="/music"
                preload="intent"
                class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <FiMusic class="w-5 h-5" aria-hidden={true} />
                Music
              </Link>

              <Link
                to="/"
                preload="intent"
                class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <FiSettings class="w-5 h-5" aria-hidden={true} />
                Settings
              </Link>

              <button
                onClick={() => {
                  handleSignOut();
                  setIsMobileMenuOpen(false);
                }}
                class="w-full px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
              >
                <FiLogOut class="w-5 h-5" aria-hidden={true} />
                Sign Out
              </button>
            </Show>

            {/* Non-authenticated Navigation Links */}
            <Show when={!isAuthenticated}>
              <Link
                to="/login"
                preload="intent"
                class="px-3 py-2 rounded-md text-base font-medium text-white hover:text-yellow-300 hover:bg-blue-800/50 transition-colors flex items-center gap-3"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <FiLogIn class="w-5 h-5" aria-hidden={true} />
                Sign In
              </Link>
            </Show>
          </div>
        </div>
      </nav>
    </>
  );
}
