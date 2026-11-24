import js from "@eslint/js";
import { plugin as shadcn } from "@shadcn/lint";
import betterTailwindcss from "eslint-plugin-better-tailwindcss";
import reactHooks from "eslint-plugin-react-hooks";
import { reactRefresh } from "eslint-plugin-react-refresh";
import { defineConfig, globalIgnores } from "eslint/config";
import globals from "globals";
import tseslint from "typescript-eslint";

// The plugin lints class strings in className attributes and inside cn(),
// and in variables by name. Its default names are className(s), classes and
// style(s); the shared recipes and variant records are named differently
// (FOCUS_VISIBLE_RING_CLASS, BUTTON_VARIANTS, lightInputClassName), so they
// are listed here. A pattern must match the whole name. Setting `variables`
// replaces the defaults, hence the first three entries.
const classStrings = [{ match: "strings" }, { match: "objectValues" }];
const tailwindSettings = {
  "better-tailwindcss": {
    entryPoint: "src/assets/styles.css",
    variables: [
      ["^classNames?$", classStrings],
      ["^classes$", classStrings],
      ["^styles?$", classStrings],
      ["^.*_(?:CLASS|CLASSES|VARIANTS|SIZES)$", classStrings],
      ["^.*[a-z](?:Class|Classes|ClassName|Variants)$", classStrings],
    ],
  },
};

const externalClassAllowlist = [
  "fa-.*",
  "lucide-.*",
  "animate-in",
  "animate-out",
  // Set on <html> by lib/theme.ts and styled in styles.css; not a utility.
  "theme-switching",
];

// Primitives whose look is chosen by props (variant, size, tone,
// elevation). The classes that would restate it (a colour, a shadow, a
// height) are banned at the call site by shadcn/no-restyle below.
const styledByProps = [
  "Button",
  "Input",
  "Textarea",
  "NativeSelect",
  "SelectTrigger",
  "DialogContent",
  "DialogTitle",
  "DialogDescription",
  "AlertDialogContent",
  "AlertDialogTitle",
  "AlertDialogDescription",
  "Spinner",
  "Label",
  "FieldHint",
  "FieldError",
  "Card",
  "CardTitle",
  "Badge",
  "DropdownMenuItem",
  "DropdownMenuContent",
  "SelectItem",
  "SelectContent",
  "PopoverContent",
].join("|");

// Class constants that set no look, and so may be passed to a primitive: the
// scrolling-body dialog recipe (docs/design-system.md section 1.6), the
// motion constants (section 1.5) and the shared focus rings (section 1.7).
const layoutClassConstants =
  "DIALOG_SCROLL_[A-Z]+_CLASS|MOTION_[A-Z_]+_CLASS|(?:PEER_|CARD_)?FOCUS_[A-Z_]+_CLASS";

// Variants are plain records (docs/design-system.md §1.6); the package was
// removed and must not come back through a copied snippet.
const cvaImportBan = {
  name: "class-variance-authority",
  message:
    "Write variants as a typed record plus cn(); see docs/design-system.md section 1.6.",
};

const restrictedClassRule = [
  "error",
  // shadcn/no-raw-colors reads className, cn() and same-file variables;
  // these two also catch an exported class constant and a template literal.
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
  // The app-wide layers and the motion durations are tokens
  // (src/lib/theme-tokens.ts): z-chrome / z-modal / z-popover and
  // duration-micro / duration-standard / duration-page. Their numbers must
  // not come back as literals, with or without the important modifier
  // (`z-50!`); see docs/design-system.md sections 1.4, 1.5.
  {
    selector:
      "Literal[value=/(?:^|[\\s:])!?(?:z-(?:40|50|60|100)|duration-(?:150|200|300))!?(?:$|\\s)/]",
    message:
      "Use the layer and duration tokens (z-chrome, z-modal, z-popover; duration-micro, duration-standard, duration-page) instead of their numbers; see docs/design-system.md sections 1.4 and 1.5.",
  },
  {
    selector:
      "TemplateElement[value.cooked=/(?:^|[\\s:])!?(?:z-(?:40|50|60|100)|duration-(?:150|200|300))!?(?:$|\\s)/]",
    message:
      "Use the layer and duration tokens instead of their numbers; see docs/design-system.md sections 1.4 and 1.5.",
  },
  // Nine dialog footers once passed `gap-2 sm:gap-0`, which left their
  // buttons touching from the `sm` breakpoint up. The footer sets its own
  // gap.
  {
    selector:
      "JSXOpeningElement[name.name=/^(?:Alert)?DialogFooter$/] > JSXAttribute[name.name='className'] Literal[value=/(?:^|\\s)(?:[a-z-]+:)*gap-/]",
    message:
      "DialogFooter sets the gap between its buttons; do not override it. See docs/design-system.md section 1.6.",
  },
  // A class constant passed to a primitive: shadcn/no-restyle cannot read
  // a constant imported from another file, so this rule names them.
  {
    selector: `JSXOpeningElement[name.name=/^(?:${styledByProps})$/] > JSXAttribute[name.name='className'] Identifier[name=/(?:ClassName|_CLASS)$/]:not([name=/^(?:${layoutClassConstants})$/])`,
    message:
      "Set the look with the primitive's props instead of a class constant. See docs/design-system.md section 1.6.",
  },
  // A raw <button> that dresses itself with buttonVariants() is a Button with
  // the wrapper left off: it misses the data-slot hooks and any later change
  // to the primitive's own markup. (An anchor or Link may still borrow the
  // look.)
  {
    selector:
      "JSXOpeningElement[name.name='button'] > JSXAttribute[name.name='className'] CallExpression[callee.name='buttonVariants']",
    message:
      "Use <Button variant=... size=...> instead of a raw <button> with buttonVariants(); see docs/design-system.md section 1.6.",
  },
];

// @shadcn/lint reads className, cn() and same-file class variables on the
// components imported from components/ui (docs/design-system.md section
// 2.1). Its theme is src/assets/styles.css, the stylesheet that imports
// Tailwind.
const shadcnSettings = {
  shadcn: { componentImports: ["@/components/ui"], mergeFunctions: ["cn"] },
};

const shadcnCategories = [
  "layout",
  "color",
  "typography",
  "spacing",
  "shape",
  "effects",
  "motion",
];

// Sizes, positions, grid templates and transition property lists have no
// scale to fall back on, so they may be arbitrary. Type, spacing, radius,
// tracking and shadows come from the theme (src/lib/theme-tokens.ts).
const arbitraryValueAllowlist = [
  "w-*",
  "min-w-*",
  "max-w-*",
  "h-*",
  "min-h-*",
  "max-h-*",
  "size-*",
  "top-*",
  "left-*",
  "translate-*",
  "-translate-*",
  "grid-cols-*",
  "grid-rows-*",
  "transition-*",
  "content-*",
  // The shared focus ring (docs/design-system.md section 1.7).
  "ring-[3px]",
];

const shadcnRules = {
  // Its report names the nearest theme tokens (docs/design-system.md
  // section 1.2).
  "shadcn/no-raw-colors": "error",
  // Any component from components/ui takes any class, except the ones whose
  // look is chosen by props: those take layout, spacing, type, shape and
  // motion, but no colour, no shadow and no fixed height. See
  // docs/design-system.md section 1.6.
  "shadcn/no-restyle": [
    "error",
    {
      allow: shadcnCategories,
      contracts: [
        {
          pattern: `^(?:${styledByProps})$`,
          // Effects minus shadows: the glow under a primary control is the
          // one shadow a call site writes (docs/design-system.md section 1.4).
          allow: [
            "layout",
            "typography",
            "spacing",
            "shape",
            "motion",
            "opacity-*",
            "shadow-glow",
          ],
          // A glob's `*` needs at least one character, so `h-4` is listed
          // beside `h-4*` (which covers `h-40`, `h-4.5`).
          deny: Array.from({ length: 10 }, (_, digit) => [
            `h-${digit}`,
            `h-${digit}*`,
          ]).flat(),
        },
      ],
    },
  ],
  "shadcn/no-arbitrary-values": ["error", { allow: arbitraryValueAllowlist }],
  // Values that change at runtime go through a CSS custom property
  // (style={{ "--progress": ... }} with w-(--progress)); see
  // docs/design-system.md section 2.1.
  "shadcn/no-inline-styles": "error",
};

// A button is a Button (docs/design-system.md section 1.6). The primitives
// and the playback surfaces draw their own (the PLAYER_* transport recipes,
// section 3.5); anywhere else a raw <button> says why inline: a whole row or
// tile that is the control, or a control over media.
const rawButtonRule = {
  selector: "JSXOpeningElement[name.name='button']",
  message:
    "Use <Button variant=... size=...> instead of a raw <button>; see docs/design-system.md section 1.6. A whole-row or whole-tile target, or a control over media, may stay raw with an inline exemption that says why.",
};

export default defineConfig([
  {
    linterOptions: {
      reportUnusedDisableDirectives: "error",
      reportUnusedInlineConfigs: "error",
    },
  },

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
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: false },
      ],
      // `const { as, ...rest } = props` drops a prop from a spread.
      "@typescript-eslint/no-unused-vars": [
        "error",
        { ignoreRestSiblings: true },
      ],
      // A route's beforeLoad throws TanStack Router's redirect() and
      // notFound(), which are not Error objects.
      "@typescript-eslint/only-throw-error": [
        "error",
        {
          allow: [
            {
              from: "package",
              package: "@tanstack/router-core",
              name: ["Redirect", "NotFoundError"],
            },
          ],
        },
      ],
      "@typescript-eslint/switch-exhaustiveness-check": [
        "error",
        {
          considerDefaultExhaustiveForUnions: true,
        },
      ],
      "@typescript-eslint/consistent-type-exports": "error",
      // A deprecated API names its replacement; the next major removes it.
      "@typescript-eslint/no-deprecated": "error",
      "@typescript-eslint/no-meaningless-void-operator": "error",
      "@typescript-eslint/no-unnecessary-boolean-literal-compare": "error",
      "@typescript-eslint/no-unnecessary-type-arguments": "error",
      "@typescript-eslint/no-unnecessary-type-conversion": "error",
      "@typescript-eslint/no-unsafe-enum-assignment": "error",
      // `||` on a string or boolean usually means "empty counts as missing",
      // and `if (ref.current === null)` is React's lazy-ref pattern.
      "@typescript-eslint/prefer-nullish-coalescing": [
        "error",
        {
          ignoreBooleanCoercion: true,
          ignoreIfStatements: true,
          ignorePrimitives: { string: true, boolean: true },
        },
      ],
      "@typescript-eslint/prefer-optional-chain": "error",
      "@typescript-eslint/prefer-reduce-type-parameter": "error",
    },
  },

  // Unit tests: the promise rules catch an assertion that never runs; the
  // rest of the type-checked preset mostly flags vi mocks.
  {
    files: ["src/test/**/*.{ts,tsx}"],
    plugins: { "react-hooks": reactHooks },
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/await-thenable": "error",
      "@typescript-eslint/no-deprecated": "error",
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: false },
      ],
      "@typescript-eslint/no-unnecessary-type-assertion": "error",
      "react-hooks/rules-of-hooks": "error",
    },
  },

  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/test/**/*.{ts,tsx}", "src/routeTree.gen.ts"],
    extends: [
      reactHooks.configs.flat["recommended-latest"],
      // TanStack Router's plugin hot-reloads a route module's `Route`.
      reactRefresh.configs.vite({
        allowExportNames: ["Route", "buttonVariants"],
      }),
      betterTailwindcss.configs["recommended-warn"],
    ],
    plugins: { shadcn },
    settings: { ...tailwindSettings, ...shadcnSettings },
    rules: {
      ...shadcnRules,
      "react-hooks/exhaustive-deps": "error",
      "react-hooks/unsupported-syntax": "error",
      "react-hooks/incompatible-library": "error",
      // The compiler skips a component it cannot lower (a `todo`) or
      // whose code breaks one of its assumptions (an `invariant`), and
      // says so only here: that component then runs unmemoized.
      "react-hooks/todo": "error",
      "react-hooks/invariant": "error",
      "better-tailwindcss/enforce-consistent-line-wrapping": "off",
      "better-tailwindcss/no-unknown-classes": [
        "warn",
        { ignore: externalClassAllowlist },
      ],
      "no-restricted-syntax": [...restrictedClassRule, rawButtonRule],
      "no-restricted-imports": [
        "error",
        {
          paths: [
            cvaImportBan,
            // Toasts go through lib/toast-helpers.ts, which fixes their
            // wording and options in one place (docs/design-system.md §3.6).
            {
              name: "sonner",
              message:
                "Fire toasts through @/lib/toast-helpers (showSuccess, showActionFailed, showInfo, ...); see docs/design-system.md section 3.6.",
            },
            // A mutation still in flight at sign-out would otherwise run its
            // callbacks against the next account's cache.
            {
              name: "@tanstack/react-query",
              importNames: ["useMutation"],
              message:
                "Use useSessionMutation from @/hooks/useSessionMutation: it drops callbacks of a mutation whose session has ended.",
            },
          ],
        },
      ],
    },
  },

  // The two files that wrap the toast library, and the tests that mock it.
  {
    files: [
      "src/lib/toast-helpers.ts",
      "src/components/ui/sonner.tsx",
      "src/test/**/*.{ts,tsx}",
    ],
    rules: {
      "no-restricted-imports": ["error", { paths: [cvaImportBan] }],
    },
  },

  // The primitives and the playback surfaces draw their own buttons.
  {
    files: [
      "src/components/ui/**/*.tsx",
      "src/components/playback/**/*.tsx",
      "src/routes/_auth/trailer.tsx",
    ],
    rules: { "no-restricted-syntax": restrictedClassRule },
  },

  {
    files: ["src/lib/input-styles.ts", "src/routes/login.tsx"],
    rules: {
      "no-restricted-syntax": "off",
      "shadcn/no-raw-colors": "off",
      "shadcn/no-restyle": "off",
    },
  },

  // A route module exports only `Route`; TanStack Router's code splitter
  // moves its components into their own chunk, which is what hot-reloads.
  {
    files: ["src/routes/**/*.tsx"],
    rules: { "react-refresh/only-export-components": "off" },
  },

  {
    files: [
      "vite.config.ts",
      "playwright.config.ts",
      "playwright.visual.config.ts",
      "scripts/**/*.ts",
    ],
    languageOptions: {
      globals: globals.node,
    },
  },

  // Every Playwright assertion and action returns a promise: one missing
  // `await` lets the test end before the check runs, so it can never fail.
  {
    files: ["e2e/**/*.ts"],
    extends: [tseslint.configs.recommendedTypeChecked],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      "@typescript-eslint/no-deprecated": "error",
      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: false },
      ],
    },
  },

  // Specs take test/expect from the shared fixture, which resets the mock and
  // tracks browser issues (e2e/e2e-test.ts); the base ones skip both.
  {
    files: ["e2e/**/*.spec.ts", "e2e/**/*.visual.ts"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "@playwright/test",
              importNames: ["test", "expect"],
              message: "Import test and expect from e2e/e2e-test.ts.",
            },
          ],
        },
      ],
    },
  },

  {
    files: ["eslint.config.js", "scripts/**/*.mjs"],
    extends: [js.configs.recommended],
    languageOptions: {
      sourceType: "module",
      globals: { ...globals.node, Bun: "readonly" },
    },
  },

  {
    files: ["**/*.{js,jsx,cjs,mjs,ts,tsx,mts,cts}"],
    rules: {
      eqeqeq: ["error", "smart"],
      curly: ["error", "multi-line"],
    },
  },
]);
