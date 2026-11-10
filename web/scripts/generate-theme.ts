// Renders the theme sync points from src/lib/theme-tokens.ts (the single
// token source — docs/design-system.md §2.4) into the marked GENERATED blocks
// in styles.css (the Tailwind theme map and the token rules), boot.css, and
// index.html.
//
//   bun run generate:theme          rewrite the generated blocks in place
//   bun run generate:theme --check  exit 1 if any block is stale (CI-friendly)
//
// The pure render functions are also imported by src/test/shared/theme-drift.test.ts,
// which fails whenever a generated block no longer matches its renderer.

import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  BOOT_COLORS,
  ELEVATION_TOKENS,
  MOTION_DURATIONS_MS,
  ROOT_ONLY_TOKENS,
  SHADOW_SCALE,
  THEME_STORAGE_KEY,
  THEME_TOKENS,
  tokenHex,
  Z_INDEX_SCALE,
  type ThemeName,
  type ThemeToken,
} from "../src/lib/theme-tokens";

const GENERATED_HINT =
  "edit src/lib/theme-tokens.ts and run `bun run generate:theme`";

function renderDeclarations(tokens: Record<string, ThemeToken>): string {
  return Object.entries(tokens)
    .map(([name, token]) => {
      const comment = token.comment ? ` /* ${token.comment} */` : "";
      return `  ${name}: ${token.value};${comment}`;
    })
    .join("\n");
}

/** The :root (light) and .dark token rules for src/assets/styles.css. */
export function renderStylesBlock(): string {
  return [
    ":root {",
    "  /* Light igloo palette — icy glacier on a cool near-white canvas. Mirrors",
    "     the .dark token set below (the dark igloo theme, the default). Both",
    "     themes are contrast-verified by src/test/shared/contrast.test.ts; see",
    "     docs/design-system.md §1.2. The app boots from the stored browser",
    "     theme, defaulting to dark. */",
    renderDeclarations({
      ...ROOT_ONLY_TOKENS,
      ...THEME_TOKENS.light,
      ...ELEVATION_TOKENS.light,
    }),
    "}",
    "",
    ".dark {",
    "  /* Igloo palette — cool glacier primary + sparing amber aurora. */",
    renderDeclarations({ ...THEME_TOKENS.dark, ...ELEVATION_TOKENS.dark }),
    "}",
  ].join("\n");
}

/**
 * The `@theme inline` rule for src/assets/styles.css: what turns the tokens
 * into Tailwind utilities (`bg-card`, `shadow-floating`, `z-modal`,
 * `duration-micro`). Every color token gets a line, so adding a token to the
 * module is all it takes for its utility to exist.
 */
export function renderThemeMapBlock(): string {
  const line = (name: string, value: string | number) => `  ${name}: ${value};`;
  return [
    "@theme inline {",
    line("--radius-sm", "calc(var(--radius) - 4px)"),
    line("--radius-md", "calc(var(--radius) - 2px)"),
    line("--radius-lg", "var(--radius)"),
    line("--radius-xl", "calc(var(--radius) + 4px)"),
    ...Object.keys(THEME_TOKENS.light).map((name) =>
      line(`--color-${name.slice(2)}`, `var(${name})`),
    ),
    ...Object.entries(SHADOW_SCALE).map(([name, value]) =>
      line(`--shadow-${name}`, value),
    ),
    ...Object.entries(Z_INDEX_SCALE).map(([name, value]) =>
      line(`--z-index-${name}`, value),
    ),
    ...Object.entries(MOTION_DURATIONS_MS).map(([name, value]) =>
      line(`--transition-duration-${name}`, `${value}ms`),
    ),
    "}",
  ].join("\n");
}

/** "#RRGGBB" -> "12, 34, 56" for rgba() gradient stops. */
function hexToRgbTriple(hex: string): string {
  return [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16))
    .join(", ");
}

function renderBodyBackground(theme: ThemeName): string {
  // Both themes tint the top of the canvas with the dark glacier primary.
  const glacier = hexToRgbTriple(tokenHex("dark", "--primary"));
  const alpha = BOOT_COLORS.glacierTintAlpha[theme];
  const radial = `radial-gradient(circle at top, rgba(${glacier}, ${alpha}), transparent 32%)`;
  const wash =
    theme === "light"
      ? // Sidebar-tinted wash down to the canvas.
        `linear-gradient(180deg, ${tokenHex("light", "--sidebar").toLowerCase()} 0%, ${tokenHex("light", "--background").toLowerCase()} 100%)`
      : `linear-gradient(180deg, rgba(${hexToRgbTriple(tokenHex("dark", "--secondary"))}, 0.94) 0%, rgba(${hexToRgbTriple(tokenHex("dark", "--background"))}, 1) 100%)`;
  return `  background:\n    ${radial},\n    ${wash};`;
}

/** Pre-hydration theme paint (canvas colors + gradients) for boot.css. */
export function renderBootBlock(): string {
  const canvas = (theme: ThemeName) =>
    [
      `  background: ${tokenHex(theme, "--background").toLowerCase()};`,
      `  color: ${tokenHex(theme, "--foreground").toLowerCase()};`,
    ].join("\n");
  return [
    "/* Light igloo palette. Dark overrides follow under html.dark; the app defaults to dark. */",
    "html {",
    canvas("light"),
    "}",
    "",
    "html.dark {",
    canvas("dark"),
    "}",
    "",
    "body {",
    renderBodyBackground("light"),
    `  color: ${tokenHex("light", "--foreground").toLowerCase()};`,
    "}",
    "",
    "html.dark body {",
    renderBodyBackground("dark"),
    `  color: ${tokenHex("dark", "--foreground").toLowerCase()};`,
    "}",
    "",
    ".initial-splash__message {",
    `  color: ${BOOT_COLORS.splashMessage.light.toLowerCase()};`,
    "}",
    "",
    "html.dark .initial-splash__message {",
    `  color: ${BOOT_COLORS.splashMessage.dark.toLowerCase()};`,
    "}",
  ].join("\n");
}

/** theme-color meta + anti-flash script for index.html (2-space indented). */
export function renderIndexBlock(): string {
  const light = tokenHex("light", "--background");
  const dark = tokenHex("dark", "--background");
  return [
    `  <meta name="theme-color" content="${dark}" />`,
    "  <!-- Anti-flash: apply the stored theme before any CSS paints. Defaults",
    "       to dark, including on storage errors. -->",
    "  <script>",
    "    (function () {",
    "      try {",
    `        var light = localStorage.getItem("${THEME_STORAGE_KEY}") === "light";`,
    '        document.documentElement.classList.toggle("dark", !light);',
    "        var meta = document.querySelector('meta[name=\"theme-color\"]');",
    `        if (meta) meta.setAttribute("content", light ? "${light}" : "${dark}");`,
    "      } catch (e) {",
    '        document.documentElement.classList.add("dark");',
    "      }",
    "    })();",
    "  </script>",
  ].join("\n");
}

export interface GeneratedTarget {
  /** What the block holds; unique, since one file can hold several. */
  name: string;
  /** Path relative to the web/ project root. */
  relPath: string;
  begin: string;
  end: string;
  render: () => string;
}

export const GENERATED_TARGETS: GeneratedTarget[] = [
  {
    name: "theme map",
    relPath: "src/assets/styles.css",
    begin: `/* BEGIN GENERATED: theme map — ${GENERATED_HINT} */`,
    end: "/* END GENERATED: theme map */",
    render: renderThemeMapBlock,
  },
  {
    name: "theme tokens",
    relPath: "src/assets/styles.css",
    begin: `/* BEGIN GENERATED: theme tokens — ${GENERATED_HINT} */`,
    end: "/* END GENERATED: theme tokens */",
    render: renderStylesBlock,
  },
  {
    name: "theme boot colors",
    relPath: "src/assets/boot.css",
    begin: `/* BEGIN GENERATED: theme boot colors — ${GENERATED_HINT} */`,
    end: "/* END GENERATED: theme boot colors */",
    render: renderBootBlock,
  },
  {
    name: "theme boot",
    relPath: "index.html",
    begin: `  <!-- BEGIN GENERATED: theme boot — ${GENERATED_HINT} -->`,
    end: "  <!-- END GENERATED: theme boot -->",
    render: renderIndexBlock,
  },
];

/** The current content between a target's markers (without the markers). */
export function extractGeneratedBlock(
  content: string,
  target: GeneratedTarget,
): string {
  const begin = content.indexOf(target.begin);
  const end = content.indexOf(target.end);
  if (begin === -1 || end === -1) {
    throw new Error(`missing GENERATED markers in ${target.relPath}`);
  }
  return content
    .slice(begin + target.begin.length, end)
    .replace(/^\n/, "")
    .replace(/\n$/, "");
}

function spliceGeneratedBlock(content: string, target: GeneratedTarget): string {
  const begin = content.indexOf(target.begin);
  const end = content.indexOf(target.end);
  if (begin === -1 || end === -1) {
    throw new Error(`missing GENERATED markers in ${target.relPath}`);
  }
  return (
    content.slice(0, begin + target.begin.length) +
    "\n" +
    target.render() +
    "\n" +
    content.slice(end)
  );
}

function main(check: boolean): void {
  const webRoot = fileURLToPath(new URL("..", import.meta.url));
  const stale: string[] = [];
  for (const target of GENERATED_TARGETS) {
    const path = resolve(webRoot, target.relPath);
    const content = readFileSync(path, "utf8");
    if (extractGeneratedBlock(content, target) === target.render()) {
      continue;
    }
    if (check) {
      stale.push(`${target.name} (${target.relPath})`);
      continue;
    }
    writeFileSync(path, spliceGeneratedBlock(content, target));
    console.log(`generated ${target.name} in ${target.relPath}`);
  }
  if (stale.length > 0) {
    console.error(
      `stale generated theme blocks (run \`bun run generate:theme\`): ${stale.join(", ")}`,
    );
    process.exit(1);
  }
}

const isMain =
  process.argv[1] != null &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) {
  main(process.argv.includes("--check"));
}
