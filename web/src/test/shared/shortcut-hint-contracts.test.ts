import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";

/**
 * A keyboard-shortcut hint is a parenthesized key list closing a label:
 * "Mute (M)", "Pause (Space or K)", "Rewind (J or Left Arrow)". Hints must go
 * through `useShortcutHints().withShortcut` so a touch-first device drops them
 * (design-system §1.7); a literal one would be read out there regardless.
 */
const KEY = String.raw`(?:(?:Ctrl|Cmd|Shift|Alt|Option)\+)*(?:[A-Z]|Space|Escape|Esc|Enter|Home|End|Tab|(?:Up|Down|Left|Right) Arrow)`;
const SHORTCUT_HINT_SUFFIX = new RegExp(
  String.raw`\(${KEY}(?: or ${KEY})*\)\s*$`,
);

function findHardCodedHints(name: string, source: string) {
  const violations: string[] = [];
  const sourceFile = ts.createSourceFile(
    name,
    source,
    ts.ScriptTarget.Latest,
    true,
    name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );

  const checkLiteral = (text: string) => {
    if (SHORTCUT_HINT_SUFFIX.test(text)) {
      violations.push(`${name}: "${text.trim()}" hard-codes a shortcut hint`);
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
    if (
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node) ||
      ts.isJsxText(node)
    ) {
      checkLiteral(node.text);
    }
    ts.forEachChild(node, visit);
  };

  visit(sourceFile);
  return violations;
}

describe("shortcut hint contracts", () => {
  it("catches every shape a hard-coded hint takes", () => {
    const hardCoded = [
      `const label = "Mute (M)";`,
      `const label = "Close trailer (Escape)";`,
      `const label = playing ? "Pause (Space or K)" : "Play";`,
      "const label = `Rewind ${step} seconds (J or Left Arrow)`;",
      "const label = `${base} (F)`;",
      `const label = "Toggle sidebar (Ctrl+B)";`,
      `const el = <button>Mute (M)</button>;`,
    ];

    for (const source of hardCoded) {
      expect(findHardCodedHints("drift.tsx", source), source).toHaveLength(1);
    }
  });

  it("leaves labels without a hint alone", () => {
    const clean = [
      `const label = "Fullscreen";`,
      `const label = "Seek backward 10 seconds";`,
      `const href = "javascript:alert(1)";`,
      `const label = "Episode 4 (Director's Cut)";`,
      `const label = "Volume (50%)";`,
      // The hook's own formatter: the key list arrives as data.
      "const label = `${label} (${keys})`;",
    ];

    for (const source of clean) {
      expect(findHardCodedHints("clean.tsx", source), source).toEqual([]);
    }
  });

  it("keeps every source file free of hard-coded shortcut hints", () => {
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
      violations.push(...findHardCodedHints(name, readFileSync(path, "utf8")));
    }

    expect(violations).toEqual([]);
  });
});
