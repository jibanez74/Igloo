import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { PLAYBACK_SETTINGS_KEY } from "@/lib/constants";
import { playbackSettingsQueryOpts } from "@/lib/query-opts";
import type { ApiResponseType, PlaybackSettingsResponseType } from "@/types";

function playbackSettingsResponse(
  preferredProfile: string,
): ApiResponseType<PlaybackSettingsResponseType> {
  return {
    error: false,
    data: {
      settings: {
        profiles: [],
        preferred_profile: preferredProfile,
        download_mbps: null,
        server_upload_mbps: null,
        is_admin: false,
        preferred_audio_language: null,
        preferred_subtitle_language: null,
      },
    },
  };
}

describe("playbackSettingsQueryOpts", () => {
  it("scopes the query key by user id", () => {
    expect(playbackSettingsQueryOpts(1).queryKey).toEqual([
      PLAYBACK_SETTINGS_KEY,
      1,
    ]);
    expect(playbackSettingsQueryOpts(2).queryKey).toEqual([
      PLAYBACK_SETTINGS_KEY,
      2,
    ]);
  });

  it("does not return one user's cached playback settings for another user", () => {
    const queryClient = new QueryClient();
    const userOneQuery = playbackSettingsQueryOpts(1);
    const userTwoQuery = playbackSettingsQueryOpts(2);
    const userOneSettings = playbackSettingsResponse("1080p_6mbps");
    const userTwoSettings = playbackSettingsResponse("720p_3mbps");

    queryClient.setQueryData(userOneQuery.queryKey, userOneSettings);

    expect(queryClient.getQueryData(userTwoQuery.queryKey)).toBeUndefined();

    queryClient.setQueryData(userTwoQuery.queryKey, userTwoSettings);

    expect(queryClient.getQueryData(userOneQuery.queryKey)).toBe(
      userOneSettings,
    );
    expect(queryClient.getQueryData(userTwoQuery.queryKey)).toBe(
      userTwoSettings,
    );
  });
});
