import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import betterTailwindcss from "eslint-plugin-better-tailwindcss";
import tseslint from "typescript-eslint";
import { defineConfig, globalIgnores } from "eslint/config";

/**
 * ESLint Configuration for Igloo
 *
 * This is the single source of truth for all ESLint rules.
 * Uses ESLint 9's flat config format.
 *
 * Includes:
 * - JavaScript/TypeScript recommended rules
 * - React Hooks rules with STRICT React Compiler enforcement
 * - React Refresh for Vite HMR
 * - Tailwind CSS v4 validation for JSX/TSX class attributes
 *
 * React Compiler Strict Mode:
 * All React Compiler rules are set to 'error' to enforce best practices.
 * Use eslint-disable comments sparingly for legitimate exceptions.
 */

export default defineConfig([
  globalIgnores(["dist"]),

  // TypeScript/React files
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      // Use recommended-latest for React Compiler support
      // See: https://react.dev/learn/react-compiler/installation#eslint-integration
      reactHooks.configs.flat["recommended-latest"],
      reactRefresh.configs.vite,
    ],
    plugins: {
      "better-tailwindcss": betterTailwindcss,
    },
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    settings: {
      "better-tailwindcss": {
        // Path to main CSS file with Tailwind v4 theme
        entryPoint: "src/assets/styles.css",
      },
    },
    rules: {
      // Allow exporting both components and utilities from the same file
      // This is a common pattern with shadcn/ui components (e.g., Button + buttonVariants)
      "react-refresh/only-export-components": [
        "warn",
        { allowExportNames: ["buttonVariants"] },
      ],

      // ============================================
      // REACT COMPILER STRICT MODE
      // All rules enforced as errors for best performance practices
      // ============================================

      // Upgrade warnings to errors for strict mode
      "react-hooks/exhaustive-deps": "error",
      "react-hooks/unsupported-syntax": "error",

      // These are already 'error' in recommended-latest, but explicitly stated for clarity:
      // - react-hooks/rules-of-hooks: error
      // - react-hooks/immutability: error
      // - react-hooks/purity: error
      // - react-hooks/refs: error
      // - react-hooks/set-state-in-render: error
      // - react-hooks/set-state-in-effect: error
      // - react-hooks/static-components: error
      // - react-hooks/globals: error

      // Tailwind CSS v4 rules (eslint-plugin-better-tailwindcss)
      // Stylistic rules (warnings) - auto-fixable
      "better-tailwindcss/enforce-consistent-class-order": "warn",
      "better-tailwindcss/no-duplicate-classes": "warn",
      "better-tailwindcss/no-unnecessary-whitespace": "warn",
      "better-tailwindcss/enforce-canonical-classes": "warn",
      "better-tailwindcss/no-deprecated-classes": "warn",

      // Disabled: Line wrapping is too aggressive for this codebase
      "better-tailwindcss/enforce-consistent-line-wrapping": "off",

      // Correctness rules (warnings) - allow Font Awesome and other external classes
      "better-tailwindcss/no-unknown-classes": [
        "warn",
        {
          // Allow external/plugin classes:
          // - Font Awesome icons (fa-*)
          // - Lucide icons (lucide-*)
          // - tw-animate-css (animate-in, animate-out)
          // - tailwind-scrollbar plugin (scrollbar-*)
          ignore: [
            "fa-.*",
            "lucide-.*",
            "animate-in",
            "animate-out",
            "scrollbar-.*",
          ],
        },
      ],
      "better-tailwindcss/no-conflicting-classes": "warn",
    },
  },
]);
