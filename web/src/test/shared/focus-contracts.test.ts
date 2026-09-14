import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import {
  CARD_FOCUS_WITHIN_RING_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  PEER_FOCUS_VISIBLE_RING_CLASS,
} from "@/lib/constants";

/**
 * Files allowed to declare their own ring, with the reason (design-system
 * §1.7). Everything else composes one of the three shared recipes.
 */
const ALLOWED = new Map([
  // The sanctioned whole-card `focus-within:` variant itself, plus the two
  // `focus-visible:` recipes, all live here.
  ["lib/constants.ts", "declares the shared ring recipes"],
  // Vendored shadcn sidebar: rings on its own --sidebar-ring token set, which
  // is deliberately separate from the app ring.
  ["components/ui/sidebar.tsx", "uses the sidebar's own --sidebar-ring token"],
]);

/** The keyboard-only variants the shared recipe is allowed to ride on. */
const FOCUS_VISIBLE_VARIANTS = new Set([
  "focus-visible",
  "focus-within",
  "peer-focus-visible",
  "group-focus-within",
]);

/**
 * Splits a Tailwind class into its variant chain and the utility it decorates:
 * `dark:focus-visible:ring-4` → `[["dark", "focus-visible"], "ring-4"]`. Only
 * colons outside brackets separate, so `supports-[display:grid]:ring-4` and
 * `[&:hover]:ring-4` survive intact — and so does the whole chain, which a
 * regex anchored on `focus-visible:` would miss the moment anything (`dark:`,
 * `md:`) is prefixed to it.
 */
function splitClassToken(token: string): [string[], string] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;

  for (let i = 0; i < token.length; i++) {
    const ch = token[i];
    if (ch === "[" || ch === "(") depth++;
    else if (ch === "]" || ch === ")") depth--;
    else if (ch === ":" && depth === 0) {
      parts.push(token.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(token.slice(start));

  return [parts.slice(0, -1), parts[parts.length - 1]];
}

/**
 * A hand-rolled ring is any focus-state `ring-*` utility that is not part of
 * the shared recipe. The recipe is `ring-[3px]` with `ring-ring/50`; anything
 * declaring a different width, a `ring-offset-*`, or a `focus:` (rather than
 * `focus-visible:`) prefix has drifted. Widths reach Tailwind four ways — bare
 * `ring` (1px), `ring-<n>`, `ring-px`, and an arbitrary `ring-[…]` — so match
 * the utility shape rather than a digit, or `ring-[4px]` slips through.
 */
const RING_WIDTH = /^ring-(?:\d+|px|\[[^\]]*\])$/;

function ringDrift(token: string): string | undefined {
  const [variants, utility] = splitClassToken(token);
  const isRing = utility === "ring" || utility.startsWith("ring-");

  if (
    variants.includes("focus") &&
    (isRing || utility === "border-ring" || utility.startsWith("border-ring/"))
  ) {
    return "uses a `focus:` ring — the shared recipe is keyboard-only (`focus-visible:`)";
  }

  if (!isRing) return;
  if (!variants.some(variant => FOCUS_VISIBLE_VARIANTS.has(variant))) return;

  if (utility.startsWith("ring-offset-")) {
    return "adds a ring offset — the shared recipe has none";
  }
  if (utility === "ring" || (RING_WIDTH.test(utility) && utility !== "ring-[3px]")) {
    return "declares a ring width other than the shared `ring-[3px]`";
  }
  if (/^ring-(?:inherit|current|transparent)$/.test(utility)) {
    return "declares a ring color outside the shared `ring-ring/50`";
  }
}

function findRingViolations(name: string, source: string) {
  const violations: string[] = [];
  const sourceFile = ts.createSourceFile(
    name,
    source,
    ts.ScriptTarget.Latest,
    true,
    name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );

  const checkLiteral = (text: string) => {
    for (const token of text.split(/\s+/)) {
      const reason = ringDrift(token);
      if (!reason) continue;
      const snippet = text.length > 60 ? `${text.slice(0, 60)}…` : text;
      violations.push(`${name}: "${snippet}" ${reason}`);
      return;
    }
  };

  const visit = (node: ts.Node) => {
    if (ts.isTemplateExpression(node)) {
      checkLiteral(
        [
          node.head.text,
          ...node.templateSpans.map((span) => span.literal.text),
        ].join(""),
      );
      for (const span of node.templateSpans) visit(span.expression);
      return;
    }
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      checkLiteral(node.text);
    }
    ts.forEachChild(node, visit);
  };

  visit(sourceFile);
  return violations;
}

describe("focus contracts", () => {
  it("pins the three shared ring recipes", () => {
    expect(FOCUS_VISIBLE_RING_CLASS).toBe(
      "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-hidden",
    );
    expect(PEER_FOCUS_VISIBLE_RING_CLASS).toBe(
      "peer-focus-visible:border-ring peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50",
    );
    // The whole-card variant is the one deliberate exception: it rings the
    // <article> on focus-within so the card shows focus wherever it lands.
    expect(CARD_FOCUS_WITHIN_RING_CLASS).toContain("focus-within:ring-2");
  });

  it("suppresses the browser outline with outline-hidden, never outline-none", () => {
    // Rings are box-shadows, which forced-colors mode strips; outline-hidden
    // keeps a transparent outline the OS can make visible there.
    expect(FOCUS_VISIBLE_RING_CLASS).toContain("focus-visible:outline-hidden");
    expect(FOCUS_VISIBLE_RING_CLASS).not.toContain("outline-none");
  });

  it("catches every shape a hand-rolled ring width takes", () => {
    // Tailwind spells a ring width four ways, and a variant chain can carry
    // any prefix ahead of the focus one. Each of these once passed the check.
    const drifted = [
      "focus-visible:ring-[4px]",
      "focus-visible:ring-px",
      "focus-visible:ring",
      "dark:focus-visible:ring-4",
      "md:group-focus-within:ring-2",
      "focus-within:ring-offset-2",
      "focus:ring-[3px]",
      "focus:border-ring",
    ];

    for (const className of drifted) {
      expect(
        findRingViolations("drift.tsx", `const c = "${className}";`),
        className,
      ).toHaveLength(1);
    }
  });

  it("leaves the recipe and its sanctioned companions alone", () => {
    // Colors and modifiers are not widths: the destructive button and the
    // softer input ring ride on the shared width, and `ring-inset` only moves
    // the same ring inward.
    const allowed = [
      FOCUS_VISIBLE_RING_CLASS,
      PEER_FOCUS_VISIBLE_RING_CLASS,
      "focus-visible:ring-inset",
      "focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40",
      "focus-visible:border-ring/70 focus-visible:ring-ring/20",
      "focus-within:border-ring/40",
      // No focus variant at all - a static ring is not a focus indicator.
      "ring-2 ring-sidebar-ring",
    ];

    for (const className of allowed) {
      expect(
        findRingViolations("clean.tsx", `const c = "${className}";`),
        className,
      ).toEqual([]);
    }
  });

  it("keeps every source file on a shared ring recipe", () => {
    // Nothing stops a component hand-writing a ring, so scan every source file
    // rather than trusting review. The check is per string literal, so one
    // allowed declaration can't exempt the rest of a file.
    const srcDir = resolve(process.cwd(), "src");
    const sourceFiles = readdirSync(srcDir, {
      recursive: true,
      withFileTypes: true,
    })
      .filter((entry) => entry.isFile() && /\.tsx?$/.test(entry.name))
      .map((entry) => resolve(entry.parentPath, entry.name))
      .filter((path) => !path.startsWith(resolve(srcDir, "test")))
      .filter((path) => path !== resolve(srcDir, "routeTree.gen.ts"));

    const violations: string[] = [];
    for (const path of sourceFiles) {
      const name = path.slice(srcDir.length + 1);
      if (ALLOWED.has(name)) continue;
      violations.push(...findRingViolations(name, readFileSync(path, "utf8")));
    }

    expect(violations).toEqual([]);
  });
});
