import type { ReactNode } from "react";
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ShowDetailsHeroActions from "@/components/shows/ShowDetailsHeroActions";
import { jsonResponse } from "../helpers/api";
import { nullableFloat64 } from "../helpers/fixtures";
import { renderWithQueryClient } from "../helpers/render";
import { SHOW_ID, episode, seasonEpisodes } from "../helpers/show-details";

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    Link: ({
      children,
      params,
      search,
      to,
      ...props
    }: {
      children: ReactNode;
      params?: { id?: string; episodeId?: string };
      search?: unknown;
      to?: string;
    }) => {
      void search;
      const href =
        typeof to === "string"
          ? to
              .replace("$id", params?.id ?? "")
              .replace("$episodeId", params?.episodeId ?? "")
          : "#";

      return (
        <a href={href} {...props}>
          {children}
        </a>
      );
    },
  };
});

function stubSeason(episodes: ReturnType<typeof episode>[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      jsonResponse({
        error: false,
        data: { season: seasonEpisodes(1).season, episodes },
      }),
    ),
  );
}

describe("ShowDetailsHeroActions", () => {
  it("plays the first episode of a fresh season", async () => {
    stubSeason([episode(1, 1), episode(1, 2)]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    const link = await screen.findByRole("link", {
      name: "Play S1 E1 S1 Episode 1",
    });
    expect(link).toHaveTextContent("Play S1 E1");
    expect(link).toHaveAttribute(
      "href",
      `/tv-shows/${SHOW_ID}/episodes/70101/play`,
    );
  });

  it("resumes the first partly watched episode ahead of unwatched ones", async () => {
    stubSeason([
      episode(1, 1, { watched: true }),
      episode(1, 2),
      episode(1, 3, {
        progress_sec: nullableFloat64(600),
        duration_sec: nullableFloat64(2700),
      }),
    ]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    const link = await screen.findByRole("link", {
      name: "Resume S1 E3 S1 Episode 3",
    });
    expect(link).toHaveTextContent("Resume S1 E3");
  });

  it("skips watched episodes when nothing is in progress", async () => {
    stubSeason([episode(1, 1, { watched: true }), episode(1, 2)]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    expect(
      await screen.findByRole("link", { name: "Play S1 E2 S1 Episode 2" }),
    ).toBeInTheDocument();
  });

  it("falls back to the first episode of a fully watched season", async () => {
    stubSeason([episode(1, 1, { watched: true }), episode(1, 2, { watched: true })]);

    renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    expect(
      await screen.findByRole("link", { name: "Play S1 E1 S1 Episode 1" }),
    ).toBeInTheDocument();
  });

  it("renders nothing for a season with no episodes", async () => {
    stubSeason([]);

    const { container } = renderWithQueryClient(
      <ShowDetailsHeroActions showId={SHOW_ID} selectedSeason={1} />,
    );

    // The query resolves asynchronously; give it a turn before asserting.
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
