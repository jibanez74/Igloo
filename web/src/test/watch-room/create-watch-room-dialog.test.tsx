import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CreateWatchRoomDialog from "@/components/watch-room/CreateWatchRoomDialog";
import {
  MOVIE_TECHNICAL_DETAILS_KEY,
  PLAYBACK_SETTINGS_SUMMARY_LOADING,
  WATCH_ROOM_INVITE_USERS_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  CreateWatchRoomResponseType,
  MovieTechnicalDetailsResponse,
  WatchRoomInviteUsersResponseType,
} from "@/types";
import type { SubtitleType } from "@/types/movies";
import { nullableString } from "../helpers/fixtures";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  audioStream,
  subtitleStream,
  videoStream,
} from "../helpers/tech-details";

const createWatchRoomMock = vi.fn();
const getMovieTechnicalDetailsMock = vi.fn();
const navigateMock = vi.fn();
const showActionFailedMock = vi.fn();
const showCreatedMock = vi.fn();
const showValidationErrorMock = vi.fn();

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    createWatchRoom: (...args: unknown[]) => createWatchRoomMock(...args),
    getMovieTechnicalDetails: (...args: unknown[]) =>
      getMovieTechnicalDetailsMock(...args),
  };
});

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
    showCreated: (...args: unknown[]) => showCreatedMock(...args),
    showValidationError: (...args: unknown[]) =>
      showValidationErrorMock(...args),
  };
});

function success<T extends Record<string, unknown>>(
  data: T,
): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function failure(message: string): ApiResponseType<never> {
  return {
    error: true,
    message,
  };
}

// The default English SDH subtitle this dialog's fixtures carry.
function sdhSubtitle(overrides: Partial<SubtitleType> = {}): SubtitleType {
  return subtitleStream({
    stream_index: 3,
    title: nullableString("SDH"),
    is_default: true,
    ...overrides,
  });
}

function technicalDetails(): ApiResponseType<MovieTechnicalDetailsResponse> {
  return success({
    movie: {
      file_name: "arrival.mp4",
      size: 1000,
      container: "mp4",
      mime_type: "video/mp4",
      run_time: { Int64: 116, Valid: true },
      duration: { Float64: 6960, Valid: true },
    },
    video_streams: [videoStream()],
    audio_streams: [
      audioStream(),
      audioStream({
        id: 2,
        stream_index: 2,
        language: { String: "spa", Valid: true },
      }),
    ],
    subtitles: [sdhSubtitle()],
    chapters: [],
  });
}

function inviteUsers(): ApiResponseType<WatchRoomInviteUsersResponseType> {
  return success({
    users: [
      {
        id: 2,
        name: "Dana Scully",
        email: "dana@example.com",
        avatar: null,
      },
      {
        id: 3,
        name: "Fox Mulder",
        email: "fox@example.com",
        avatar: "/api/static/avatars/fox.webp",
      },
    ],
  });
}

type DialogProps = ComponentProps<typeof CreateWatchRoomDialog>;

function renderDialog(
  overrides: Partial<DialogProps> = {},
  { seedTechnicalDetails = true } = {},
) {
  const queryClient = createTestQueryClient();
  queryClient.setQueryData([WATCH_ROOM_INVITE_USERS_KEY], inviteUsers());
  if (seedTechnicalDetails) {
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
  }

  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const onOpenChange = vi.fn();
  const props: DialogProps = {
    movieId: 22,
    movieTitle: "Arrival",
    playbackSettings: {
      mode: "direct",
      audioTrack: 1,
      subtitleTrack: 0,
    },
    open: true,
    onOpenChange,
    ...overrides,
  };

  return {
    invalidateSpy,
    onOpenChange,
    ...renderWithQueryClient(<CreateWatchRoomDialog {...props} />, {
      queryClient,
    }),
  };
}

beforeEach(() => {
  createWatchRoomMock.mockReset();
  getMovieTechnicalDetailsMock.mockReset();
  getMovieTechnicalDetailsMock.mockReturnValue(new Promise(() => {}));
  navigateMock.mockReset();
  showActionFailedMock.mockReset();
  showCreatedMock.mockReset();
  showValidationErrorMock.mockReset();
});

describe("CreateWatchRoomDialog", () => {
  it("filters invitees by name or email without losing the selected users", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByText("Dana Scully")).toBeInTheDocument();
    expect(screen.getByText("Fox Mulder")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Search users"), "fox@example");

    expect(screen.queryByText("Dana Scully")).not.toBeInTheDocument();
    expect(screen.getByText("Fox Mulder")).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: /invite fox mulder/i }));
    expect(screen.getByText("1 selected")).toBeInTheDocument();

    await user.clear(screen.getByLabelText("Search users"));

    expect(
      screen.getByRole("button", { name: /remove fox mulder from invited users/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("Dana Scully")).toBeInTheDocument();
  });

  it("requires at least one invited user before creating a room", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Create and join room" }));

    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "Select at least one person to invite.",
    );
    expect(createWatchRoomMock).not.toHaveBeenCalled();
  });

  // Before technical details resolve, getAvailableModes optimistically keeps
  // `direct`, so an early submit would post a mode the source may not support
  // and the server would reject the room. The presets section has to say why
  // the button is disabled, or it reads as broken.
  it("cannot create a room until technical details resolve", async () => {
    const user = userEvent.setup();
    renderDialog({}, { seedTechnicalDetails: false });

    await user.click(
      screen.getByRole("checkbox", { name: /invite dana scully/i }),
    );

    expect(
      screen.getByRole("button", { name: "Create and join room" }),
    ).toBeDisabled();
    expect(screen.getByRole("status")).toHaveTextContent(
      PLAYBACK_SETTINGS_SUMMARY_LOADING,
    );
    expect(createWatchRoomMock).not.toHaveBeenCalled();
  });

  it("reports a technical-details failure instead of leaving the button silently disabled", async () => {
    getMovieTechnicalDetailsMock.mockResolvedValue(
      failure("Technical details are unavailable."),
    );
    renderDialog({}, { seedTechnicalDetails: false });

    expect(
      await screen.findByText("Technical details are unavailable."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create and join room" }),
    ).toBeDisabled();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("names the direct-play audio language, as the player's badge does", () => {
    renderDialog({
      playbackSettings: { mode: "direct", audioTrack: 0, subtitleTrack: null },
    });

    const playback = screen.getByText("Playback").nextElementSibling;
    expect(playback).toHaveTextContent("Original file — English audio");
  });

  it("creates the room with resolved playback settings and navigates to it", async () => {
    createWatchRoomMock.mockResolvedValue(
      success<CreateWatchRoomResponseType>({ room_id: 123 }),
    );

    const user = userEvent.setup();
    const { invalidateSpy, onOpenChange } = renderDialog();

    await user.click(screen.getByRole("checkbox", { name: /invite dana scully/i }));
    await user.click(screen.getByRole("button", { name: "Create and join room" }));

    await waitFor(() => {
      // The room is asked for the second audio track, which direct playback
      // cannot deliver, so the resolved settings send remux instead.
      expect(createWatchRoomMock).toHaveBeenCalledWith({
        movie_id: 22,
        mode: "remux",
        audio_track: 1,
        subtitle_track: 0,
        invited_user_ids: [2],
      });
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: [WATCH_ROOMS_KEY],
    });
    expect(showCreatedMock).toHaveBeenCalledWith(
      "Watch room",
      "\"Arrival\" is ready to watch together.",
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(navigateMock).toHaveBeenCalledWith({
      to: "/watch-rooms/$id",
      params: { id: 123 },
    });
  });

  it("keeps the dialog open and reports API errors", async () => {
    createWatchRoomMock.mockResolvedValue({
      error: true,
      message: "No HLS profile is available.",
    });

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    await user.click(screen.getByRole("checkbox", { name: /invite dana scully/i }));
    await user.click(screen.getByRole("button", { name: "Create and join room" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "create watch room",
        "No HLS profile is available.",
      );
    });

    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(navigateMock).not.toHaveBeenCalled();
    expect(screen.getByText("Watch together")).toBeInTheDocument();
  });
});
