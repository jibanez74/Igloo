import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { authUser, nullableFloat64, nullableInt64, nullableString } from "../helpers/fixtures";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";
import { SHOW_ID } from "../helpers/show-details";

// The player pauses the app-wide audio player on mount, so the route needs
// its provider; the direct stream is never fetched in jsdom, so no media
// element stubs are required beyond what the loader and header queries need.
function renderEpisodeRoute(path: string) {
  return renderRoute(path, {
    wrapper: children => <AudioPlayerProvider>{children}</AudioPlayerProvider>,
  });
}

const EPISODE_ID = 70103;

const episodePayload = {
  show: {
    id: SHOW_ID,
    name: "Frost Harbor",
    poster_path: nullableString("/frost-harbor.jpg"),
    backdrop_path: nullableString("/frost-harbor-backdrop.jpg"),
  },
  season: { season_number: 1, name: "Season 1" },
  episode: {
    id: EPISODE_ID,
    episode_number: 3,
    name: "The Thaw",
    overview: nullableString("The ice gives way."),
    air_date: nullableString("2024-03-22"),
    still_path: nullableString("/still.jpg"),
    tmdb_runtime: nullableInt64(47),
    vote_average: nullableFloat64(8.1),
    vote_count: nullableInt64(220),
  },
};

const technicalDetails = {
  file: {
    file_name: "Frost.Harbor.S01E03.mp4",
    size: 1_200_000_000,
    container: "mp4",
    mime_type: "video/mp4",
    duration: nullableFloat64(2700),
  },
  video_streams: [
    {
      id: 1,
      file_id: 5,
      stream_index: 0,
      codec: "h264",
      codec_profile: nullableString("High"),
      codec_level: nullableInt64(41),
      bit_rate: 6_000_000,
      width: 1920,
      height: 1080,
      coded_width: nullableInt64(1920),
      coded_height: nullableInt64(1080),
      aspect_ratio: nullableString("16:9"),
      frame_rate: 23.976,
      avg_frame_rate: nullableString("24000/1001"),
      bit_depth: nullableInt64(8),
      pixel_format: nullableString("yuv420p"),
      color_range: nullableString("tv"),
      color_space: nullableString("bt709"),
      color_primaries: nullableString("bt709"),
      color_transfer: nullableString("bt709"),
      field_order: nullableString("progressive"),
      rotation: nullableInt64(null),
      language: nullableString("eng"),
      title: nullableString(""),
    },
  ],
  audio_streams: [
    {
      id: 2,
      file_id: 5,
      stream_index: 1,
      codec: "aac",
      codec_profile: nullableString("LC"),
      bit_rate: 192_000,
      sample_rate: nullableInt64(48_000),
      channels: 2,
      channel_layout: nullableString("stereo"),
      language: nullableString("eng"),
      title: nullableString(""),
      is_default: true,
    },
  ],
  subtitles: [],
  chapters: [],
};

function mockEpisodeApi(options: { episodeStatus?: number } = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") return jsonResponse(authUser());
    if (url === "/api/notifications/unread-count") {
      return jsonResponse({ error: false, data: { unread_count: 0 } });
    }
    if (url === "/api/settings/playback") {
      return jsonResponse({
        error: false,
        data: { settings: { profiles: [], server_upload_mbps: null } },
      });
    }
    if (url === `/api/shows/episodes/${EPISODE_ID}`) {
      if (options.episodeStatus) {
        return jsonResponse(
          { error: true, message: "episode not found" },
          options.episodeStatus,
        );
      }
      return jsonResponse({ error: false, data: episodePayload });
    }
    if (url === `/api/shows/episodes/${EPISODE_ID}/technical-details`) {
      return jsonResponse({ error: false, data: technicalDetails });
    }
    if (url === `/api/shows/episodes/${EPISODE_ID}/watch-progress`) {
      return jsonResponse({
        error: false,
        data: {
          progress_sec: null,
          duration_sec: null,
          watched: false,
          updated_at: null,
        },
      });
    }

    return jsonResponse(
      { error: true, message: `Unexpected request: ${url}` },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("episode play route", () => {
  it("titles the player after the show, episode code, and episode name", async () => {
    mockEpisodeApi();

    const { router } = await renderEpisodeRoute(
      `/tv-shows/${SHOW_ID}/episodes/${EPISODE_ID}/play?mode=direct&audio_track=0&subtitle_track=off&start=0`,
    );

    expect(
      await screen.findByRole("region", {
        name: "Video player for Frost Harbor · S1 E3 · The Thaw",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Frost Harbor · S1 E3 · The Thaw" }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(
      `/tv-shows/${SHOW_ID}/episodes/${EPISODE_ID}/play`,
    );
  });

  it("canonicalizes a link without a mode into the episode's default settings", async () => {
    mockEpisodeApi();

    const { router } = await renderEpisodeRoute(
      `/tv-shows/${SHOW_ID}/episodes/${EPISODE_ID}/play?start=0`,
    );

    await screen.findByRole("region", { name: /Video player for/ });
    const search = router.state.location.search as Record<string, unknown>;
    expect(search.mode).toBe("direct");
    expect(search.audio_track).toBe(0);
    expect(search.subtitle_track).toBe("off");
  });

  it("reports an unknown episode in the player's not-found copy", async () => {
    mockEpisodeApi({ episodeStatus: 404 });

    await renderEpisodeRoute(
      `/tv-shows/${SHOW_ID}/episodes/${EPISODE_ID}/play?mode=direct&audio_track=0&subtitle_track=off&start=0`,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Episode not found",
    );
    expect(
      screen.getByText(/The episode could not be found/),
    ).toBeInTheDocument();
  });
});
