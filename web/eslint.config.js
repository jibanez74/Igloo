import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import betterTailwindcss from "eslint-plugin-better-tailwindcss";
import tseslint from "typescript-eslint";
import { defineConfig, globalIgnores } from "eslint/config";

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
// elevation), and the classes that would restate it: a colour on the fill,
// text, border or shadow, or a height.
const styledByProps = [
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
  "Card",
  "CardTitle",
  "Badge",
  "DropdownMenuItem",
  "DropdownMenuContent",
  "SelectItem",
  "SelectContent",
  "PopoverContent",
].join("|");
const lookClass =
  "(?:^|\\s)(?:[a-z-]+:)*(?:(?:bg|text|border|shadow|placeholder:text)-(?:background|foreground|card|popover|primary|secondary|muted|accent|destructive|border|input|ring|success|aurora|white|black|transparent)|h-[0-9])";

// Class constants that set no look, and so may be passed to a primitive: the
// scrolling-body dialog recipe (docs/design-system.md section 1.6), the
// motion constants (section 1.5) and the shared focus rings (section 1.7).
const layoutClassConstants =
  "DIALOG_SCROLL_[A-Z]+_CLASS|MOTION_[A-Z_]+_CLASS|(?:PEER_|CARD_)?FOCUS_[A-Z_]+_CLASS";

const restrictedClassRule = [
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
  // A primitive's look comes from its props, not from classes or a class
  // constant at the call site. `className` is for layout: margins, padding
  // around an icon, width, scrolling, a font. The list grows as each
  // primitive family takes its looks in; see docs/design-system.md
  // section 1.6.
  {
    selector: `JSXOpeningElement[name.name=/^(?:${styledByProps})$/] > JSXAttribute[name.name='className'] :matches(Literal[value=/${lookClass}/], TemplateElement[value.cooked=/${lookClass}/])`,
    message:
      "Set the look with the primitive's props (variant, size, tone, elevation) instead of classes; className is for layout. See docs/design-system.md section 1.6.",
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
      "no-restricted-syntax": restrictedClassRule,
      // Variants are plain records (docs/design-system.md §1.6); the package
      // was removed and must not come back through a copied snippet.
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "class-variance-authority",
              message:
                "Write variants as a typed record plus cn(); see docs/design-system.md section 1.6.",
            },
          ],
        },
      ],
    },
  },

  {
    files: ["src/lib/input-styles.ts", "src/routes/login.tsx"],
    rules: { "no-restricted-syntax": "off" },
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

  {
    files: ["**/*.{js,jsx,cjs,mjs,ts,tsx,mts,cts}"],
    rules: {
      eqeqeq: ["error", "smart"],
      curly: ["error", "multi-line"],
    },
  },
]);
