import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ChapterMenu from "@/components/movies/ChapterMenu";
import type { ChapterType } from "@/types";

const nullString = { String: "", Valid: false };
const nullInt64 = { Int64: 0, Valid: false };

function makeChapter(overrides: Partial<ChapterType>): ChapterType {
  return {
    id: 0,
    title: "",
    start_time: 0,
    thumb: nullString,
    movie_id: nullInt64,
    ...overrides,
  };
}

const chapters: ChapterType[] = [
  makeChapter({ id: 1, title: "Opening Credits", start_time: 0 }),
  makeChapter({ id: 2, title: "", start_time: 83 }),
  makeChapter({ id: 3, title: "The Heist", start_time: 3923 }),
];

describe("ChapterMenu", () => {
  it("renders nothing when there are no chapters", () => {
    const { container } = render(
      <ChapterMenu
        chapters={[]}
        currentTimeSec={0}
        onSelectChapter={vi.fn()}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("labels the trigger with a pluralized chapter count", () => {
    render(
      <ChapterMenu
        chapters={chapters}
        currentTimeSec={0}
        onSelectChapter={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Chapters, 3 chapters" }),
    ).toBeInTheDocument();
  });

  it("gives each item a spoken label, falling back to Chapter N and marking the current chapter", async () => {
    const user = userEvent.setup();

    render(
      <ChapterMenu
        chapters={chapters}
        currentTimeSec={100}
        onSelectChapter={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Chapters, 3 chapters" }));

    expect(
      await screen.findByRole("menuitem", {
        name: "Chapter 1 of 3, Opening Credits, starts at 0 seconds",
      }),
    ).toBeInTheDocument();

    // Untitled chapters fall back to "Chapter N" and read the start time in
    // words. currentTimeSec=100 falls within this chapter, so it is current.
    const fallback = screen.getByRole("menuitem", {
      name: "Chapter 2 of 3, starts at 1 minute 23 seconds, current chapter",
    });
    expect(fallback).toHaveAttribute("aria-current", "true");

    // Long-form start times include the hour in the spoken label.
    expect(
      screen.getByRole("menuitem", {
        name: "Chapter 3 of 3, The Heist, starts at 1 hour 5 minutes 23 seconds",
      }),
    ).toBeInTheDocument();
  });

  it("passes the resolved label when an untitled chapter is selected", async () => {
    const user = userEvent.setup();
    const onSelectChapter = vi.fn();

    render(
      <ChapterMenu
        chapters={chapters}
        currentTimeSec={0}
        onSelectChapter={onSelectChapter}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Chapters, 3 chapters" }));
    await user.click(
      await screen.findByRole("menuitem", {
        name: "Chapter 2 of 3, starts at 1 minute 23 seconds",
      }),
    );

    expect(onSelectChapter).toHaveBeenCalledWith(83, "Chapter 2");
  });
});
