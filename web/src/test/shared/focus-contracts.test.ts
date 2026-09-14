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

/**
 * A hand-rolled ring is any focus-state `ring-*` utility that is not part of
 * the shared recipe. The recipe is `ring-[3px]` with `ring-ring/50`; anything
 * declaring a different width, a `ring-offset-*`, or a `focus:` (rather than
 * `focus-visible:`) prefix has drifted.
 */
const DRIFT_PATTERNS: [RegExp, string][] = [
  [
    /(?:^|\s)focus:(?:ring-|border-ring\b)/,
    "uses a `focus:` ring — the shared recipe is keyboard-only (`focus-visible:`)",
  ],
  [
    /(?:^|\s)(?:focus-visible|focus-within|peer-focus-visible|group-focus-within):ring-(?!\[3px\])(?:\d|inherit|current|transparent)/,
    "declares a ring width other than the shared `ring-[3px]`",
  ],
  [
    /(?:^|\s)(?:focus-visible|focus-within|peer-focus-visible):ring-offset-/,
    "adds a ring offset — the shared recipe has none",
  ],
];

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
    for (const [pattern, reason] of DRIFT_PATTERNS) {
      if (!pattern.test(text)) continue;
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
