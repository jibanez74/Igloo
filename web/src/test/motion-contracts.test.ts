import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import {
  CARD_ACTION_REVEAL_CLASS,
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_DECORATIVE_BOUNCE_CLASS,
  MOTION_DECORATIVE_PING_CLASS,
  MOTION_DURATION_MICRO_MS,
  MOTION_DURATION_PAGE_MS,
  MOTION_DURATION_STANDARD_MS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MEDIA_DIALOG_SURFACE_CLASS,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_MEDIA_OVERLAY_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  MOTION_MICRO_OPACITY_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_PAGE_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  MOTION_PROGRESS_FILL_CLASS,
  MOTION_PROGRESS_THUMB_REVEAL_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  MOTION_SPINNER_STATE_CLASS,
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";

function durationClass(durationMs: number) {
  return `duration-${durationMs}`;
}

function findInlineMotionViolations(name: string, source: string) {
  const violations: string[] = [];
  const sourceFile = ts.createSourceFile(
    name,
    source,
    ts.ScriptTarget.Latest,
    true,
    name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const checkLiteral = (text: string) => {
    const snippet = text.length > 60 ? `${text.slice(0, 60)}…` : text;
    if (
      /(?<!motion-reduce:)transition-(?!none\b)/.test(text) &&
      !/motion-reduce:transition-(none|colors)\b/.test(text)
    ) {
      violations.push(
        `${name}: "${snippet}" declares a transition without a motion-reduce:transition-none/colors fallback`,
      );
    }
    if (
      /(?<!motion-reduce:)animate-(?!none\b)/.test(text) &&
      !text.includes("motion-reduce:animate-none")
    ) {
      violations.push(
        `${name}: "${snippet}" declares an animation without motion-reduce:animate-none`,
      );
    }
  };
  const visit = (node: ts.Node) => {
    if (ts.isTemplateExpression(node)) {
      const text = [
        node.head.text,
        ...node.templateSpans.map((span) => span.literal.text),
      ].join("");
      checkLiteral(text);
      for (const span of node.templateSpans) {
        visit(span.expression);
      }
      return;
    }
    if (
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node)
    ) {
      checkLiteral(node.text);
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return violations;
}

describe("motion contracts", () => {
  it("exports shared duration tokens", () => {
    expect(MOTION_DURATION_MICRO_MS).toBe(150);
    expect(MOTION_DURATION_STANDARD_MS).toBe(200);
    expect(MOTION_DURATION_PAGE_MS).toBe(300);
    expect(CONTENT_FADE_TRANSITION_MS).toBe(MOTION_DURATION_STANDARD_MS);
  });

  it("keeps existing contracts reduced-motion safe", () => {
    for (const className of [
      DETAIL_PAGE_CONTENT_ENTER_CLASS,
      CARD_INTERACTIVE_SURFACE_CLASS,
      CARD_SURFACE_CLASS,
      CARD_MEDIA_HOVER_CLASS,
      CARD_OVERLAY_REVEAL_CLASS,
      CARD_ACTION_REVEAL_CLASS,
      CONTENT_FADE_ENTER_CLASS,
      CONTENT_FADE_EXIT_CLASS,
    ]) {
      expect(className).toContain("motion-reduce:");
    }

    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(DETAIL_PAGE_CONTENT_ENTER_CLASS).toContain(
      "motion-reduce:translate-y-0",
    );
    expect(CONTENT_FADE_ENTER_CLASS).toContain("motion-reduce:animate-none");
    expect(CONTENT_FADE_ENTER_CLASS).toContain("motion-reduce:opacity-100");
    expect(CARD_SURFACE_CLASS).toContain(CARD_INTERACTIVE_SURFACE_CLASS);
  });

  it("exports reduced-motion behavior for shared contracts", () => {
    expect(MOTION_MICRO_CONTROL_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_MICRO_OPACITY_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_MICRO_OPACITY_CLASS).toContain(
      durationClass(MOTION_DURATION_MICRO_MS),
    );
    expect(MOTION_MICRO_COLORS_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_MICRO_COLORS_CLASS).toContain(
      durationClass(MOTION_DURATION_MICRO_MS),
    );
    expect(MOTION_PROGRESS_FILL_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_PROGRESS_FILL_CLASS).toContain(
      durationClass(MOTION_DURATION_MICRO_MS),
    );
    expect(MOTION_PROGRESS_THUMB_REVEAL_CLASS).toBe(
      MOTION_MICRO_OPACITY_CLASS,
    );
    expect(MOTION_CONTROL_THUMB_TRANSFORM_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_CONTROL_THUMB_TRANSFORM_CLASS).toContain(
      durationClass(MOTION_DURATION_STANDARD_MS),
    );
    expect(MOTION_SETTINGS_SURFACE_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_SETTINGS_SURFACE_CLASS).toContain(
      durationClass(MOTION_DURATION_STANDARD_MS),
    );
    expect(MOTION_TRACK_ROW_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_TRACK_PLAY_BUTTON_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_TRACK_ICON_BUTTON_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_TRACK_MENU_TRIGGER_CLASS).toBe(MOTION_MICRO_COLORS_CLASS);
    expect(MOTION_PLAYER_CHROME_PANEL_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_PLAYER_CHROME_PANEL_CLASS).toContain(
      "motion-reduce:transform-none",
    );
    expect(MOTION_MEDIA_OVERLAY_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_MEDIA_DIALOG_SURFACE_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_MEDIA_OVERLAY_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_MEDIA_OVERLAY_ENTER_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(MOTION_MEDIA_OVERLAY_ENTER_CLASS).toContain(
      "motion-reduce:scale-100",
    );
    expect(MOTION_MEDIA_OVERLAY_ENTER_CLASS).toContain(
      "motion-reduce:translate-y-0",
    );
    expect(MOTION_MEDIA_OVERLAY_ENTER_CLASS).toContain(
      durationClass(MOTION_DURATION_STANDARD_MS),
    );
    expect(MOTION_PLAYER_CHROME_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_PLAYER_CHROME_ENTER_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(MOTION_PLAYER_CHROME_ENTER_CLASS).toContain(
      "motion-reduce:translate-y-0",
    );
    expect(MOTION_PLAYER_CHROME_ENTER_CLASS).toContain(
      durationClass(MOTION_DURATION_STANDARD_MS),
    );
    expect(MOTION_PLAYER_CHROME_BUTTON_CLASS).toBe(MOTION_MICRO_CONTROL_CLASS);
    expect(MOTION_PAGE_ENTER_CLASS).toContain("motion-reduce:animate-none");
    expect(MOTION_PAGE_ENTER_CLASS).toContain("motion-reduce:translate-y-0");
    expect(MOTION_SECTION_ENTER_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_SECTION_ENTER_DELAYED_CLASS).toContain("delay-75");
    expect(MOTION_SECTION_ENTER_DELAYED_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_SECTION_ENTER_DELAYED_CLASS).toContain(
      "motion-reduce:opacity-100",
    );
    expect(MOTION_SECTION_ENTER_DELAYED_CLASS).toContain(
      "motion-reduce:delay-0",
    );
    expect(MOTION_LOADING_STATE_CLASS).toContain("motion-reduce:animate-none");
    expect(MOTION_SPINNER_STATE_CLASS).toContain("motion-reduce:animate-none");
    expect(MOTION_DECORATIVE_PING_CLASS).toContain(
      "motion-reduce:animate-none",
    );
    expect(MOTION_DECORATIVE_BOUNCE_CLASS).toContain(
      "motion-reduce:animate-none",
    );
  });

  it("treats template expressions as one class-bearing literal", () => {
    const safeSource = [
      "const transition = `transition-opacity ${state} motion-reduce:transition-none`;",
      "const animation = `animate-pulse ${state} motion-reduce:animate-none`;",
    ].join("\n");
    expect(findInlineMotionViolations("safe.tsx", safeSource)).toEqual([]);

    const unsafeSource =
      'const classes = `opacity-100 ${active ? "animate-pulse" : ""}`;';
    expect(findInlineMotionViolations("unsafe.tsx", unsafeSource)).toEqual([
      expect.stringContaining(
        "declares an animation without motion-reduce:animate-none",
      ),
    ]);
  });

  it("keeps inline transitions and animations reduced-motion safe", () => {
    // Shared MOTION_* constants are covered above, but some files declare
    // motion classes inline — scan every source file so a new one can't slip
    // through. The check is per string literal: a class list that declares a
    // transition or animation must carry its reduced-motion fallback in the
    // same literal, so one escaped declaration can't exempt the rest of the
    // file. Transitions may downgrade to motion-reduce:transition-colors
    // instead of transition-none (color fades aren't motion).
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
      const source = readFileSync(path, "utf8");
      const name = path.slice(srcDir.length + 1);
      violations.push(...findInlineMotionViolations(name, source));
    }
    expect(violations).toEqual([]);
  });
});
