import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CreateWatchRoomDialog from "@/components/watch-room/CreateWatchRoomDialog";
import {
  MOVIE_TECHNICAL_DETAILS_KEY,
  WATCH_ROOM_INVITE_USERS_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  CreateWatchRoomResponseType,
  MovieTechnicalDetailsResponse,
  WatchRoomInviteUsersResponseType,
} from "@/types";
import type {
  AudioStreamType,
  SubtitleType,
  VideoStreamType,
} from "@/types/movies";

const createWatchRoomMock = vi.fn();
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

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: false,
      },
      queries: {
        retry: false,
      },
    },
  });
}

function success<T extends Record<string, unknown>>(
  data: T,
): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function videoStream(overrides: Partial<VideoStreamType> = {}): VideoStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 0,
    codec: "h264",
    codec_profile: { String: "Main", Valid: true },
    codec_level: { Int64: 41, Valid: true },
    bit_rate: 4_000_000,
    width: 1920,
    height: 1080,
    coded_width: { Int64: 1920, Valid: true },
    coded_height: { Int64: 1080, Valid: true },
    aspect_ratio: { String: "16:9", Valid: true },
    frame_rate: 24,
    avg_frame_rate: { String: "24/1", Valid: true },
    bit_depth: { Int64: 8, Valid: true },
    color_range: { String: "", Valid: false },
    color_space: { String: "", Valid: false },
    color_primaries: { String: "", Valid: false },
    color_transfer: { String: "", Valid: false },
    language: { String: "", Valid: false },
    title: { String: "", Valid: false },
    ...overrides,
  };
}

function audioStream(overrides: Partial<AudioStreamType> = {}): AudioStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 1,
    codec: "aac",
    codec_profile: { String: "LC", Valid: true },
    bit_rate: 192_000,
    sample_rate: { Int64: 48_000, Valid: true },
    channels: 2,
    channel_layout: { String: "stereo", Valid: true },
    language: { String: "eng", Valid: true },
    title: { String: "", Valid: false },
    ...overrides,
  };
}

function subtitleStream(
  overrides: Partial<SubtitleType> = {},
): SubtitleType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 3,
    codec: "subrip",
    language: { String: "eng", Valid: true },
    title: { String: "SDH", Valid: true },
    is_forced: false,
    is_default: true,
    ...overrides,
  };
}

function technicalDetails(): ApiResponseType<MovieTechnicalDetailsResponse> {
  return success({
    movie: {
      file_name: "arrival.mp4",
      file_path: "/media/arrival.mp4",
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
    subtitles: [subtitleStream()],
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
        avatar: "avatars/fox.webp",
      },
    ],
  });
}

type DialogProps = ComponentProps<typeof CreateWatchRoomDialog>;

function renderDialog(overrides: Partial<DialogProps> = {}) {
  const queryClient = createTestQueryClient();
  queryClient.setQueryData([WATCH_ROOM_INVITE_USERS_KEY], inviteUsers());
  queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], technicalDetails());

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

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }

  return {
    invalidateSpy,
    onOpenChange,
    queryClient,
    ...render(<CreateWatchRoomDialog {...props} />, { wrapper: Wrapper }),
  };
}

beforeEach(() => {
  createWatchRoomMock.mockReset();
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
