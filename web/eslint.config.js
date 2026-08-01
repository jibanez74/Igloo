import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import betterTailwindcss from "eslint-plugin-better-tailwindcss";
import tseslint from "typescript-eslint";
import { defineConfig, globalIgnores } from "eslint/config";

const tailwindSettings = {
  "better-tailwindcss": {
    entryPoint: "src/assets/styles.css",
  },
};

const externalClassAllowlist = [
  "fa-.*",
  "lucide-.*",
  "animate-in",
  "animate-out",
];

const rawTailwindPaletteRule = [
  "error",
  {
    selector:
      "Literal[value=/(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950)/]",
    message:
      "Use semantic tokens (bg-background, text-muted-foreground, bg-aurora, text-destructive, text-success, ...) instead of raw Tailwind palette colors; see docs/design-system.md section 1.2.",
  },
  {
    selector:
      "TemplateElement[value.cooked=/(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-(?:50|100|200|300|400|500|600|700|800|900|950)/]",
    message:
      "Use semantic tokens instead of raw Tailwind palette colors; see docs/design-system.md section 1.2.",
  },
];

export default defineConfig([
  globalIgnores([
    "dist",
    "playwright-report",
    "test-results",
    "blob-report",
    "coverage",
    ".vite",
    ".bun",
    "node_modules",
    "src/routeTree.gen.ts",
    "src/types/openapi.gen.ts",
  ]),

  {
    files: ["**/*.{ts,tsx}"],
    extends: [js.configs.recommended, tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
  },

  {
    files: ["src/**/*.{ts,tsx}", "vite.config.ts", "scripts/**/*.ts"],
    ignores: ["src/test/**/*.{ts,tsx}", "src/routeTree.gen.ts"],
    extends: [tseslint.configs.recommendedTypeChecked],
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/no-floating-promises": "off",
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: false },
      ],
      "@typescript-eslint/only-throw-error": "off",
    },
  },

  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/test/**/*.{ts,tsx}", "src/routeTree.gen.ts"],
    extends: [
      reactHooks.configs.flat["recommended-latest"],
      reactRefresh.configs.vite,
      betterTailwindcss.configs["recommended-warn"],
    ],
    settings: tailwindSettings,
    rules: {
      "react-refresh/only-export-components": [
        "warn",
        { allowExportNames: ["buttonVariants"] },
      ],
      "react-hooks/exhaustive-deps": "error",
      "react-hooks/unsupported-syntax": "error",
      "better-tailwindcss/enforce-consistent-line-wrapping": "off",
      "better-tailwindcss/no-unknown-classes": [
        "warn",
        { ignore: externalClassAllowlist },
      ],
      "no-restricted-syntax": rawTailwindPaletteRule,
    },
  },

  {
    files: ["src/lib/input-styles.ts", "src/routes/login.tsx"],
    rules: { "no-restricted-syntax": "off" },
  },

  {
    files: ["vite.config.ts", "playwright.config.ts", "scripts/**/*.ts"],
    languageOptions: {
      globals: globals.node,
    },
  },

  {
    files: ["e2e/**/*.ts"],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
  },

  {
    files: ["eslint.config.js"],
    extends: [js.configs.recommended],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: globals.node,
    },
  },
]);
