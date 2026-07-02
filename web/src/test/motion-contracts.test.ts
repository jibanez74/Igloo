import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  CARD_ACTION_REVEAL_CLASS,
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_DECORATIVE_BOUNCE_CLASS,
  MOTION_DECORATIVE_PING_CLASS,
  MOTION_DECORATIVE_STATE_CLASS,
  MOTION_DURATION_MICRO_MS,
  MOTION_DURATION_PAGE_MS,
  MOTION_DURATION_STANDARD_MS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MEDIA_DIALOG_SURFACE_CLASS,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_MEDIA_OVERLAY_CLASS,
  MOTION_MICRO_OPACITY_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  MOTION_PAGE_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  MOTION_PROGRESS_FILL_CLASS,
  MOTION_PROGRESS_THUMB_REVEAL_CLASS,
  MOTION_ROW_SURFACE_CLASS,
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
    expect(MOTION_TRACK_MENU_TRIGGER_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_ROW_SURFACE_CLASS).toContain(
      "motion-reduce:transition-colors",
    );
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
    expect(MOTION_DECORATIVE_STATE_CLASS).toContain(
      "motion-reduce:transition-none",
    );
    expect(MOTION_DECORATIVE_STATE_CLASS).toContain(
      "motion-reduce:transform-none",
    );
  });

  it("keeps ui primitives with inline transitions reduced-motion safe", () => {
    // Shared MOTION_* constants are covered above, but ui/ primitives declare
    // transitions inline — scan their sources so a new one can't slip through.
    const uiDir = resolve(process.cwd(), "src/components/ui");
    const animatedFiles = readdirSync(uiDir)
      .filter((name) => name.endsWith(".tsx"))
      .filter((name) =>
        /transition-(?!none)/.test(readFileSync(resolve(uiDir, name), "utf8")),
      );
    expect(animatedFiles.length).toBeGreaterThan(0);
    for (const name of animatedFiles) {
      const source = readFileSync(resolve(uiDir, name), "utf8");
      expect(
        source,
        `${name} declares a transition without motion-reduce:transition-none`,
      ).toContain("motion-reduce:transition-none");
    }
  });
});
