import type {
  ContinueWatchingItemType,
  DeviceType,
  GeneralSettingsType,
  LatestShowType,
  MovieTechnicalDetailsResponse,
  MusicStatsType,
  PlaybackSettingsType,
  SettingsType,
  ShowEpisodePlaybackDataType,
  ShowEpisodeTechnicalDetailsDataType,
  WatchProgressType,
  TrailerPreferencesData,
} from "../src/types";
import {
  apiResponse,
  IDLE_SCAN_STATUS_BY_PATH,
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { intEnv, readE2EEnv } from "./e2e-env";
import {
  libraryMovie,
  libraryMovieDetails,
  MOCK_MOVIE_ID,
  prerollTrailers,
} from "./fixtures/movies";
import {
  libraryShow,
  MOCK_EPISODE_ID,
  MOCK_NEXT_EPISODE_ID,
  MOCK_SHOW_ID,
} from "./fixtures/shows";
import { randomUUID } from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import { MOVIES_PER_PAGE, STREAM_MODES } from "../src/lib/constants";

// The stateful API behind the specs that do not call mockApi (settings, auth,
// devices, the players, trailer, scan progress, head metadata and motion). It
// serves only what those specs reach; anything else answers a 404 that the
// specs' browser-issue checks report.

type User = {
  id: number;
  name: string;
  email: string;
  password: string;
  is_admin: boolean;
  avatar: string | null;
  created_at: string;
  updated_at: string;
};

type ServerPlaybackSettings = Pick<
  PlaybackSettingsType,
  "server_upload_mbps" | "hardware_acceleration_device"
>;

type PendingPairing = {
  secret: string;
  device_name: string;
  platform: string;
  app_version: string | null;
  approved: boolean;
};

const HOST = "127.0.0.1";
const PORT = intEnv("E2E_MOCK_API_PORT", 8080);
const SESSION_COOKIE = "igloo_e2e_session";
const CLEAR_SESSION_COOKIE = `${SESSION_COOKIE}=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax`;
const admin = readE2EEnv();
const startedAt = new Date().toISOString();

// Messages the Go handlers answer with, so a spec reading one sees the real text.
const ADMIN_REQUIRED = "admin access required";
const DUPLICATE_ADMIN_EMAIL = "a user with that email already exists";
const DUPLICATE_OWN_EMAIL = "that email address is already in use";
const INVALID_CODE = "invalid or expired code";
const USER_NOT_FOUND = "user not found";
const INVALID_CREDENTIALS = "invalid email or password provided";
const WRONG_CURRENT_PASSWORD = "current password is incorrect";
const NAME_TOO_LONG = "must be at most 100 characters";

// Both mirror UpdatePlaybackSettings in the Go server: a mock that accepts what
// the real handler answers with 400 lets a spec pass against a contract that
// does not exist.
const SERVER_UPLOAD_MAX_MBPS = 100_000;
const HARDWARE_ACCELERATION_DEVICES = [
  "cpu",
  "apple",
  "nvidia",
  "intel",
] as const;

function isHardwareAccelerationDevice(
  value: unknown,
): value is ServerPlaybackSettings["hardware_acceleration_device"] {
  return (
    typeof value === "string" &&
    (HARDWARE_ACCELERATION_DEVICES as readonly string[]).includes(value)
  );
}

const NO_PROGRESS = {
  progress_sec: null,
  duration_sec: null,
  watched: false,
  updated_at: null,
} satisfies WatchProgressType;

let nextUserId = 2;
let nextDeviceId = 1;

const sessions = new Map<string, number>();
// Quick Connect pairing state. Starts empty so specs that only render the
// settings page see no devices. Bearer-token auth on other routes is
// deliberately not simulated: token validity/revocation semantics are covered
// by the Go integration tests and the live-gated device-lifecycle spec.
const devices: DeviceType[] = [];
const pendingPairings = new Map<string, PendingPairing>();

const users: User[] = [
  {
    id: 1,
    name: "Igloo Admin",
    email: admin.email,
    password: admin.password,
    is_admin: true,
    avatar: null,
    created_at: startedAt,
    updated_at: startedAt,
  },
];

let librarySettings: SettingsType = {
  movies_dir: "/srv/media/movies",
  shows_dir: null,
  music_dir: "/srv/media/music",
};

let generalSettings: GeneralSettingsType = {
  tmdb_key: null,
  immich_base_url: null,
  immich_api_key: null,
  jellyfin_base_url: null,
  jellyfin_api_key: null,
  spotify_client_id: null,
  spotify_client_secret: null,
  enable_watcher: true,
  download_images: true,
  static_dir: "/tmp/igloo/static",
  transcode_dir: "/tmp/igloo/transcodes",
};

let serverPlaybackSettings: ServerPlaybackSettings = {
  server_upload_mbps: null,
  hardware_acceleration_device: "cpu",
};

// Account-scoped trailer pre-roll preferences, per user id; a missing entry
// is the server's default (off). The preroll route serves a fixed queue
// while the feature is on, so specs restore the entry they changed.
const DEFAULT_TRAILER_PREFERENCES: TrailerPreferencesData = {
  enabled: false,
  count: 2,
  source: "both",
};
const TRAILER_SOURCES = ["library", "theaters", "both"];
const trailerPreferences = new Map<number, TrailerPreferencesData>();

// The server's transcode catalog, labelled the way the Go handler labels it.
const playbackProfiles = STREAM_MODES.filter(mode => mode.type === "transcode").map(
  mode => {
    const videoMbps = Number(/_(\d+)mbps$/.exec(mode.id)?.[1]);
    return {
      id: mode.id,
      label: `${mode.maxHeight}p · ${videoMbps} Mbps`,
      height: mode.maxHeight,
      video_mbps: videoMbps,
    };
  },
);

const libraryMovies = [
  libraryMovie(MOCK_MOVIE_ID, "Signal Fire", 2024, "/signal-fire.jpg"),
  libraryMovie(102, "Northern Relay", 2023, "/northern-relay.jpg"),
  libraryMovie(103, "Harbor Lights", 2022, "/harbor-lights.jpg"),
];

const latestShows = [
  {
    id: MOCK_SHOW_ID,
    name: "Frost Harbor",
    poster_path: nullableString("/api/static/shows/frost-harbor.svg"),
    premiere_year: nullableInt64(2026),
  },
  {
    id: 402,
    name: "Halcyon Drift",
    poster_path: nullableString("/api/static/shows/halcyon-drift.svg"),
    premiere_year: nullableInt64(2024),
  },
] satisfies LatestShowType[];

const libraryShows = latestShows.map(show =>
  libraryShow(show.id, show.name, show.premiere_year.Int64, show.poster_path.String),
);

const latestAlbums = [
  {
    id: 201,
    title: "Warm Static",
    cover: nullableString("/api/static/albums/warm-static.svg"),
    musician: nullableString("The Signals"),
    year: nullableInt64(2024),
  },
  {
    id: 202,
    title: "Night Index",
    cover: nullableString("/api/static/albums/night-index.svg"),
    musician: nullableString("June Harbor"),
    year: nullableInt64(2023),
  },
];

const musicStats = {
  total_albums: latestAlbums.length,
  total_tracks: 3,
  total_musicians: 2,
} satisfies MusicStatsType;

const theaterMovies = [
  {
    id: 601,
    title: "Low Orbit Kitchen",
    original_title: "Low Orbit Kitchen",
    overview: "A compact crew tries to keep dinner service running in orbit.",
    release_date: "2026-05-15",
    poster_path: "/low-orbit-kitchen.jpg",
    backdrop_path: "/low-orbit-kitchen-backdrop.jpg",
    popularity: 84,
    vote_average: 7.2,
    vote_count: 183,
    adult: false,
    original_language: "en",
    genre_ids: [12, 35],
    video: false,
  },
];

function sendJSON(
  response: ServerResponse,
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
) {
  response.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "no-store",
    ...headers,
  });
  response.end(JSON.stringify(body));
}

function sendSuccess(
  response: ServerResponse,
  data: unknown = {},
  status = 200,
  message?: string,
  headers?: Record<string, string>,
) {
  sendJSON(
    response,
    status,
    { ...apiResponse(data), ...(message ? { message } : {}) },
    headers,
  );
}

function sendFailure(response: ServerResponse, status: number, message: string) {
  sendJSON(response, status, { error: true, message });
}

function sendPlaceholderImage(response: ServerResponse) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="960" viewBox="0 0 640 960"><rect width="640" height="960" fill="#0f172a"/><rect x="48" y="48" width="544" height="864" rx="24" fill="#1e293b"/><text x="320" y="480" fill="#f59e0b" font-family="Arial, sans-serif" font-size="42" font-weight="700" text-anchor="middle">Igloo</text></svg>`;
  response.writeHead(200, {
    "Content-Type": "image/svg+xml; charset=utf-8",
    "Cache-Control": "public, max-age=3600",
  });
  response.end(svg);
}

function parseCookies(request: IncomingMessage) {
  const header = request.headers.cookie ?? "";
  const cookies = new Map<string, string>();
  for (const part of header.split(";")) {
    const [name, ...valueParts] = part.trim().split("=");
    if (name) {
      cookies.set(name, decodeURIComponent(valueParts.join("=")));
    }
  }
  return cookies;
}

function currentUser(request: IncomingMessage) {
  const sessionId = parseCookies(request).get(SESSION_COOKIE);
  if (!sessionId) return null;
  const userId = sessions.get(sessionId);
  if (!userId) return null;
  return users.find(user => user.id === userId) ?? null;
}

async function readJSONBody(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  const raw = Buffer.concat(chunks).toString("utf8");
  if (!raw) return {};

  const parsed = JSON.parse(raw) as unknown;
  return parsed !== null && typeof parsed === "object" && !Array.isArray(parsed)
    ? (parsed as Record<string, unknown>)
    : {};
}

function stringField(
  body: Record<string, unknown>,
  key: string,
  fallback = "",
) {
  const value = body[key];
  return typeof value === "string" ? value : fallback;
}

function booleanField(
  body: Record<string, unknown>,
  key: string,
  fallback = false,
) {
  const value = body[key];
  return typeof value === "boolean" ? value : fallback;
}

/**
 * A trimmed string field where "" and null clear the value, and an absent
 * field keeps `current`.
 */
function nullableStringField(
  body: Record<string, unknown>,
  key: string,
  current: string | null,
) {
  const value = body[key];
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed === "" ? null : trimmed;
  }
  return value === null ? null : current;
}

function publicUser(user: User) {
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    is_admin: user.is_admin,
    avatar: user.avatar,
    // PIN changes only run against a real instance.
    has_pin: false,
    created_at: user.created_at,
    updated_at: user.updated_at,
  };
}

/** Answers 403 and returns false unless `user` is an administrator. */
function requireAdmin(user: User, response: ServerResponse) {
  if (!user.is_admin) {
    sendFailure(response, 403, ADMIN_REQUIRED);
  }
  return user.is_admin;
}

function findUserByEmail(email: string) {
  const normalizedEmail = email.trim().toLowerCase();
  return users.find(user => user.email.toLowerCase() === normalizedEmail) ?? null;
}

function findUser(response: ServerResponse, id: string) {
  const user = users.find(item => item.id === Number(id));
  if (!user) {
    sendFailure(response, 404, USER_NOT_FOUND);
  }
  return user;
}

function touchUser(user: User) {
  user.updated_at = new Date().toISOString();
}

function removeUser(userId: number) {
  const index = users.findIndex(user => user.id === userId);
  if (index >= 0) {
    users.splice(index, 1);
  }
  for (const [sessionId, sessionUserId] of sessions) {
    if (sessionUserId === userId) {
      sessions.delete(sessionId);
    }
  }
}

/**
 * One page of a library list, sorted by `nameOf` in the requested direction,
 * in the envelope the movie and show library routes share.
 */
function libraryList<K extends string, T>(
  url: URL,
  key: K,
  items: T[],
  nameOf: (item: T) => string,
) {
  const page = Math.max(1, Number.parseInt(url.searchParams.get("page") ?? "1", 10));
  const perPage = Math.max(
    1,
    Number.parseInt(url.searchParams.get("per_page") ?? String(MOVIES_PER_PAGE), 10),
  );
  const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
  const sorted = [...items].sort((a, b) => {
    const value = nameOf(a).localeCompare(nameOf(b));
    return sort === "asc" ? value : -value;
  });
  const start = (page - 1) * perPage;

  return {
    [key]: sorted.slice(start, start + perPage),
    total: items.length,
    page,
    per_page: perPage,
    total_pages: Math.max(1, Math.ceil(items.length / perPage)),
    sort,
  };
}

function movieTechnicalDetails(id: number) {
  return {
    movie: {
      file_name: "Signal Fire.mp4",
      size: 4_200_000_000,
      container: "mp4",
      mime_type: "video/mp4",
      run_time: nullableInt64(122),
      duration: nullableFloat64(7320),
    },
    video_streams: [
      {
        id: 1,
        movie_id: id,
        stream_index: 0,
        codec: "h264",
        codec_profile: nullableString("High"),
        codec_level: nullableInt64(41),
        bit_rate: 8_000_000,
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
        rotation: nullableInt64(),
        language: nullableString("eng"),
        title: nullableString("Main"),
      },
    ],
    audio_streams: [
      {
        id: 1,
        movie_id: id,
        stream_index: 1,
        codec: "aac",
        codec_profile: nullableString("LC"),
        bit_rate: 384_000,
        sample_rate: nullableInt64(48000),
        channels: 6,
        channel_layout: nullableString("5.1"),
        language: nullableString("eng"),
        title: nullableString("English"),
        is_default: true,
      },
    ],
    subtitles: [
      {
        id: 1,
        movie_id: id,
        stream_index: 2,
        codec: "subrip",
        language: nullableString("eng"),
        title: nullableString("English"),
        is_forced: false,
        is_default: false,
      },
    ],
    chapters: [
      {
        id: 1,
        title: "Opening Credits",
        start_time: 0,
        thumb: nullableString("/api/static/chapters/opening.svg"),
        movie_id: nullableInt64(id),
      },
      {
        id: 2,
        title: "The Journey",
        start_time: 372,
        thumb: nullableString(),
        movie_id: nullableInt64(id),
      },
    ],
  } satisfies MovieTechnicalDetailsResponse;
}

function playbackSettingsResponse() {
  return {
    profiles: playbackProfiles,
    ...serverPlaybackSettings,
    // The mock has no FFmpeg probe, so the stored device is the effective one
    // and only the CPU cap applies, as the Go handler reports.
    effective_hardware_acceleration_device:
      serverPlaybackSettings.hardware_acceleration_device,
    hardware_fallback_reason: "",
    max_transcode_height:
      serverPlaybackSettings.hardware_acceleration_device === "cpu" ? 1080 : 2160,
  } satisfies PlaybackSettingsType;
}

// Public quick-connect routes: the pairing device is unauthenticated until it
// redeems an approved code, mirroring the real server.
async function handleQuickConnectPublicRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/quick-connect/initiate" && method === "POST") {
    const body = await readJSONBody(request);
    const deviceName = stringField(body, "device_name").trim();
    if (!deviceName || deviceName.length > 100) {
      sendFailure(response, 400, `device_name is required and ${NAME_TOO_LONG}`);
      return true;
    }

    let code = "";
    do {
      code = randomUUID().replace(/[^A-Z2-9]/gi, "").toUpperCase().slice(0, 6);
    } while (code.length < 6 || pendingPairings.has(code));

    const secret = randomUUID();
    pendingPairings.set(code, {
      secret,
      device_name: deviceName,
      platform: stringField(body, "platform"),
      app_version: nullableStringField(body, "app_version", null),
      approved: false,
    });

    sendSuccess(
      response,
      { code, secret, expires_in_seconds: 300, poll_interval_seconds: 2 },
      201,
    );
    return true;
  }

  if (url.pathname === "/api/quick-connect/redeem" && method === "POST") {
    const body = await readJSONBody(request);
    const code = stringField(body, "code").trim().toUpperCase();
    const pairing = pendingPairings.get(code);

    if (!pairing || pairing.secret !== stringField(body, "secret")) {
      sendFailure(response, 404, INVALID_CODE);
      return true;
    }

    if (!pairing.approved) {
      sendSuccess(response, { status: "pending" });
      return true;
    }

    const now = new Date().toISOString();
    const device = {
      id: nextDeviceId++,
      name: pairing.device_name,
      platform: pairing.platform,
      app_version: pairing.app_version,
      created_at: now,
      last_used_at: now,
    };
    devices.push(device);
    pendingPairings.delete(code);

    sendSuccess(response, {
      status: "approved",
      token: `igd_mock_${randomUUID()}`,
      device,
    });
    return true;
  }

  return false;
}

async function handleAuthRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/auth/login" && method === "POST") {
    const body = await readJSONBody(request);
    const user = findUserByEmail(stringField(body, "email"));

    if (!user || user.password !== stringField(body, "password")) {
      sendFailure(response, 401, INVALID_CREDENTIALS);
      return true;
    }

    const sessionId = randomUUID();
    sessions.set(sessionId, user.id);
    sendSuccess(response, { user: publicUser(user) }, 200, "Login successful", {
      "Set-Cookie": `${SESSION_COOKIE}=${encodeURIComponent(sessionId)}; Path=/; HttpOnly; SameSite=Lax`,
    });
    return true;
  }

  if (url.pathname === "/api/auth/logout" && method === "DELETE") {
    const sessionId = parseCookies(request).get(SESSION_COOKIE);
    if (sessionId) {
      sessions.delete(sessionId);
    }
    sendSuccess(response, {}, 200, "Logged out", { "Set-Cookie": CLEAR_SESSION_COOKIE });
    return true;
  }

  if (url.pathname === "/api/auth/user" && method === "GET") {
    const user = currentUser(request);
    if (!user) {
      sendFailure(response, 401, "Unauthorized");
      return true;
    }
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  return false;
}

async function handleUserRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
  user: User,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/user/name" && method === "PUT") {
    const body = await readJSONBody(request);
    user.name = stringField(body, "name", user.name).trim();
    touchUser(user);
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  if (url.pathname === "/api/user/email" && method === "PUT") {
    const body = await readJSONBody(request);
    const email = stringField(body, "email", user.email).trim();
    const duplicate = findUserByEmail(email);
    if (duplicate && duplicate.id !== user.id) {
      sendFailure(response, 409, DUPLICATE_OWN_EMAIL);
      return true;
    }
    user.email = email;
    touchUser(user);
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  if (url.pathname === "/api/user/password" && method === "PUT") {
    const body = await readJSONBody(request);
    if (user.password !== stringField(body, "current_password")) {
      sendFailure(response, 401, WRONG_CURRENT_PASSWORD);
      return true;
    }
    user.password = stringField(body, "new_password");
    touchUser(user);
    sendSuccess(response);
    return true;
  }

  if (url.pathname === "/api/user/avatar" && method === "PUT") {
    const body = await readJSONBody(request);
    user.avatar = stringField(body, "avatar", user.avatar ?? "").trim() || null;
    touchUser(user);
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  if (url.pathname === "/api/user/preferences/trailers" && method === "GET") {
    sendSuccess(response, trailerPreferences.get(user.id) ?? DEFAULT_TRAILER_PREFERENCES);
    return true;
  }

  if (url.pathname === "/api/user/preferences/trailers" && method === "PUT") {
    const body = await readJSONBody(request);
    const enabled = body.enabled;
    const count = body.count;
    const source = body.source;
    if (typeof enabled !== "boolean" || typeof count !== "number" || typeof source !== "string") {
      sendFailure(response, 400, "invalid request body");
      return true;
    }
    if (!Number.isInteger(count) || count < 1 || count > 5) {
      sendFailure(response, 400, "trailer count must be between 1 and 5");
      return true;
    }
    if (!TRAILER_SOURCES.includes(source)) {
      sendFailure(response, 400, "trailer source must be library, theaters or both");
      return true;
    }
    const prefs = { enabled, count, source } as TrailerPreferencesData;
    trailerPreferences.set(user.id, prefs);
    sendSuccess(response, prefs);
    return true;
  }

  if (url.pathname === "/api/user" && method === "DELETE") {
    removeUser(user.id);
    sendSuccess(response, {}, 200, "Account deleted", { "Set-Cookie": CLEAR_SESSION_COOKIE });
    return true;
  }

  return false;
}

async function handleAdminRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/admin/users" && method === "GET") {
    sendSuccess(response, { users: users.map(publicUser) });
    return true;
  }

  if (url.pathname === "/api/admin/users" && method === "POST") {
    const body = await readJSONBody(request);
    const email = stringField(body, "email").trim();
    if (findUserByEmail(email)) {
      sendFailure(response, 409, DUPLICATE_ADMIN_EMAIL);
      return true;
    }
    const now = new Date().toISOString();
    const user: User = {
      id: nextUserId++,
      name: stringField(body, "name", "New User").trim(),
      email,
      password: stringField(body, "password"),
      is_admin: booleanField(body, "is_admin"),
      avatar: null,
      created_at: now,
      updated_at: now,
    };
    users.push(user);
    sendSuccess(response, { user: publicUser(user) }, 201);
    return true;
  }

  const passwordMatch = url.pathname.match(/^\/api\/admin\/users\/(\d+)\/password$/);
  if (passwordMatch && method === "PUT") {
    const target = findUser(response, passwordMatch[1]);
    if (!target) return true;
    const body = await readJSONBody(request);
    target.password = stringField(body, "password", target.password);
    touchUser(target);
    sendSuccess(response);
    return true;
  }

  const userMatch = url.pathname.match(/^\/api\/admin\/users\/(\d+)$/);
  if (userMatch && method === "PATCH") {
    const target = findUser(response, userMatch[1]);
    if (!target) return true;
    const body = await readJSONBody(request);
    const email = stringField(body, "email", target.email).trim();
    const duplicate = findUserByEmail(email);
    if (duplicate && duplicate.id !== target.id) {
      sendFailure(response, 409, DUPLICATE_ADMIN_EMAIL);
      return true;
    }
    target.name = stringField(body, "name", target.name).trim();
    target.email = email;
    target.is_admin = booleanField(body, "is_admin", target.is_admin);
    touchUser(target);
    sendSuccess(response, { user: publicUser(target) });
    return true;
  }

  if (userMatch && method === "DELETE") {
    removeUser(Number(userMatch[1]));
    sendSuccess(response);
    return true;
  }

  return false;
}

async function handleSettingsRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
  user: User,
) {
  const method = request.method ?? "GET";

  // No spec starts a scan through the mock (scan-progress stubs its own status
  // route), so the admin shell's polls always see an idle scanner.
  const scanStatus = IDLE_SCAN_STATUS_BY_PATH[url.pathname];
  if (scanStatus && method === "GET") {
    sendSuccess(response, scanStatus());
    return true;
  }

  if (url.pathname === "/api/settings" && method === "GET") {
    sendSuccess(response, librarySettings);
    return true;
  }

  if (url.pathname === "/api/settings/libraries" && method === "PUT") {
    if (!requireAdmin(user, response)) return true;
    const body = await readJSONBody(request);
    librarySettings = {
      movies_dir: nullableStringField(body, "movies_dir", librarySettings.movies_dir),
      shows_dir: nullableStringField(body, "shows_dir", librarySettings.shows_dir),
      music_dir: nullableStringField(body, "music_dir", librarySettings.music_dir),
    };
    sendSuccess(response, { settings: librarySettings });
    return true;
  }

  if (url.pathname === "/api/settings/general" && method === "GET") {
    if (!requireAdmin(user, response)) return true;
    sendSuccess(response, { settings: generalSettings });
    return true;
  }

  if (url.pathname === "/api/settings/general" && method === "PUT") {
    if (!requireAdmin(user, response)) return true;
    const body = await readJSONBody(request);
    const current = generalSettings;
    generalSettings = {
      tmdb_key: nullableStringField(body, "tmdb_key", current.tmdb_key),
      immich_base_url: nullableStringField(body, "immich_base_url", current.immich_base_url),
      immich_api_key: nullableStringField(body, "immich_api_key", current.immich_api_key),
      jellyfin_base_url: nullableStringField(body, "jellyfin_base_url", current.jellyfin_base_url),
      jellyfin_api_key: nullableStringField(body, "jellyfin_api_key", current.jellyfin_api_key),
      spotify_client_id: nullableStringField(body, "spotify_client_id", current.spotify_client_id),
      spotify_client_secret: nullableStringField(
        body,
        "spotify_client_secret",
        current.spotify_client_secret,
      ),
      enable_watcher: booleanField(body, "enable_watcher", current.enable_watcher),
      download_images: booleanField(body, "download_images", current.download_images),
      static_dir: stringField(body, "static_dir", current.static_dir),
      transcode_dir: stringField(body, "transcode_dir", current.transcode_dir),
    };
    sendSuccess(response, { settings: generalSettings, restart_required: false });
    return true;
  }

  if (url.pathname === "/api/settings/playback" && method === "GET") {
    sendSuccess(response, { settings: playbackSettingsResponse() });
    return true;
  }

  if (url.pathname === "/api/settings/playback" && method === "PUT") {
    if (!requireAdmin(user, response)) return true;
    const body = await readJSONBody(request);

    // Absent fields keep their current value; the ones that were sent are
    // validated exactly as the Go handler validates them.
    let serverUploadMbps = serverPlaybackSettings.server_upload_mbps;
    if (Object.hasOwn(body, "server_upload_mbps")) {
      const sent = body.server_upload_mbps;
      if (typeof sent === "number" && Number.isFinite(sent)) {
        if (sent <= 0 || sent >= SERVER_UPLOAD_MAX_MBPS) {
          sendFailure(
            response,
            400,
            `server upload speed must be greater than 0 and less than ${SERVER_UPLOAD_MAX_MBPS} Mbps`,
          );
          return true;
        }
        serverUploadMbps = sent;
      } else if (sent === null) {
        serverUploadMbps = null;
      }
    }

    let hardwareDevice = serverPlaybackSettings.hardware_acceleration_device;
    if (Object.hasOwn(body, "hardware_acceleration_device")) {
      const sent = body.hardware_acceleration_device;
      // Rejects null and any unknown string, so the union type is enforced by a
      // check rather than by a cast that assumes it.
      if (!isHardwareAccelerationDevice(sent)) {
        sendFailure(response, 400, "invalid hardware acceleration device");
        return true;
      }
      hardwareDevice = sent;
    }

    serverPlaybackSettings = {
      server_upload_mbps: serverUploadMbps,
      hardware_acceleration_device: hardwareDevice,
    };

    sendSuccess(response, { settings: playbackSettingsResponse() });
    return true;
  }

  return false;
}

// Two mocked episodes serve the episode player specs: the header names a
// show and season, the file mirrors the movie technical fixture keyed by
// file_id instead of movie_id, and the first episode hands off to the second
// so the up-next flow has somewhere to go.
function showEpisodePlayback(episodeId: number) {
  const isNext = episodeId === MOCK_NEXT_EPISODE_ID;
  return {
    show: {
      id: MOCK_SHOW_ID,
      name: "Frost Harbor",
      poster_path: nullableString("/frost-harbor.jpg"),
      backdrop_path: nullableString("/frost-harbor-backdrop.jpg"),
    },
    season: { season_number: 1, name: "Season 1" },
    episode: {
      id: episodeId,
      episode_number: isNext ? 4 : 3,
      name: isNext ? "The Long Night" : "The Thaw",
      overview: nullableString("The ice gives way."),
      air_date: nullableString("2026-03-22"),
      still_path: nullableString("/still.jpg"),
      tmdb_runtime: nullableInt64(47),
      vote_average: nullableFloat64(8.1),
      vote_count: nullableInt64(220),
    },
    next_episode: isNext
      ? null
      : {
          id: MOCK_NEXT_EPISODE_ID,
          season_number: 1,
          episode_number: 4,
          name: "The Long Night",
          still_path: nullableString("/next-still.jpg"),
          progress_sec: nullableFloat64(),
          duration_sec: nullableFloat64(),
          watched: false,
        },
  } satisfies ShowEpisodePlaybackDataType;
}

function episodeTechnicalDetails(episodeId: number) {
  const { movie, ...streams } = movieTechnicalDetails(MOCK_MOVIE_ID);
  const withFileId = <T extends { movie_id: unknown }>(rows: T[]) =>
    rows.map(({ movie_id, ...row }) => {
      void movie_id;
      return { ...row, file_id: episodeId };
    });
  return {
    file: {
      file_name: "Frost.Harbor.S01E03.mp4",
      size: movie.size,
      container: movie.container,
      mime_type: movie.mime_type,
      duration: movie.duration,
    },
    video_streams: withFileId(streams.video_streams),
    audio_streams: withFileId(streams.audio_streams),
    subtitles: withFileId(streams.subtitles),
    chapters: withFileId(streams.chapters),
  } satisfies ShowEpisodeTechnicalDetailsDataType;
}

function handleShowsRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/shows/latest" && method === "GET") {
    sendSuccess(response, { shows: latestShows });
    return true;
  }

  if (url.pathname === "/api/shows/stats" && method === "GET") {
    sendSuccess(response, { total_shows: libraryShows.length });
    return true;
  }

  if (url.pathname === "/api/shows/library" && method === "GET") {
    sendSuccess(response, libraryList(url, "shows", libraryShows, show => show.name));
    return true;
  }

  const episodeMatch = url.pathname.match(
    /^\/api\/shows\/episodes\/(\d+)(\/technical-details|\/watch-progress)?$/,
  );
  if (!episodeMatch) {
    return false;
  }
  const episodeId = Number(episodeMatch[1]);

  // The finished-episode test saves progress when the video ends; nothing
  // reads it back.
  if (episodeMatch[2] === "/watch-progress" && method === "PUT") {
    sendSuccess(response, { watched: false });
    return true;
  }

  if (method !== "GET") {
    return false;
  }

  if (episodeMatch[2] === "/watch-progress") {
    sendSuccess(response, NO_PROGRESS);
    return true;
  }

  if (episodeMatch[2] === "/technical-details") {
    sendSuccess(response, episodeTechnicalDetails(episodeId));
    return true;
  }

  if (episodeId !== MOCK_EPISODE_ID && episodeId !== MOCK_NEXT_EPISODE_ID) {
    sendFailure(response, 404, "episode not found");
    return true;
  }
  sendSuccess(response, showEpisodePlayback(episodeId));
  return true;
}

function handleMoviesRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
  user: User,
) {
  if ((request.method ?? "GET") !== "GET") {
    return false;
  }

  const prerollMatch = url.pathname.match(/^\/api\/movies\/(\d+)\/preroll$/);
  if (prerollMatch) {
    const movie = libraryMovies.find(item => item.id === Number(prerollMatch[1]));
    if (!movie) {
      sendFailure(response, 404, "movie not found");
      return true;
    }
    const prefs = trailerPreferences.get(user.id) ?? DEFAULT_TRAILER_PREFERENCES;
    sendSuccess(response, {
      trailers: prefs.enabled ? prerollTrailers().slice(0, prefs.count) : [],
    });
    return true;
  }

  if (url.pathname === "/api/movies/latest") {
    sendSuccess(response, { movies: libraryMovies });
    return true;
  }

  if (url.pathname === "/api/movies/stats") {
    sendSuccess(response, { total_movies: libraryMovies.length });
    return true;
  }

  if (url.pathname === "/api/movies/library") {
    sendSuccess(response, libraryList(url, "movies", libraryMovies, movie => movie.title));
    return true;
  }

  const detailsMatch = url.pathname.match(/^\/api\/movies\/details\/(\d+)$/);
  if (detailsMatch) {
    const movie = libraryMovies.find(item => item.id === Number(detailsMatch[1]));
    if (!movie) {
      sendFailure(response, 404, "movie not found");
      return true;
    }
    sendSuccess(response, libraryMovieDetails(movie));
    return true;
  }

  const technicalMatch = url.pathname.match(/^\/api\/movies\/(\d+)\/technical-details$/);
  if (technicalMatch) {
    sendSuccess(response, movieTechnicalDetails(Number(technicalMatch[1])));
    return true;
  }

  // The mocked player never plays far enough to save progress.
  if (/^\/api\/movies\/\d+\/watch-progress$/.test(url.pathname)) {
    sendSuccess(response, NO_PROGRESS);
    return true;
  }

  return false;
}

// Home is only reached when the trailer spec's Escape returns to `/`; its
// loader asks for all of these.
function handleHomeRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  if ((request.method ?? "GET") !== "GET") {
    return false;
  }

  switch (url.pathname) {
    case "/api/continue-watching":
      sendSuccess(response, {
        items: [
          {
            kind: "episode",
            id: MOCK_EPISODE_ID,
            title: latestShows[0].name,
            poster_path: latestShows[0].poster_path,
            year: latestShows[0].premiere_year,
            progress_sec: 600,
            duration_sec: 2400,
            show_id: latestShows[0].id,
            season_number: 1,
            episode_number: 3,
            episode_name: "The Thaw",
          },
          ...libraryMovies.slice(0, 2).map((movie, index) => ({
            kind: "movie" as const,
            ...movie,
            progress_sec: 900 * (index + 1),
            duration_sec: 5400,
          })),
        ] satisfies ContinueWatchingItemType[],
      });
      return true;
    case "/api/music/albums/latest":
      sendSuccess(response, { albums: latestAlbums });
      return true;
    case "/api/tmdb/movies/in-theaters":
      sendSuccess(response, { movies: theaterMovies });
      return true;
    case "/api/watch-rooms":
      sendSuccess(response, { rooms: [] });
      return true;
    default:
      return false;
  }
}

async function handleDeviceRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/devices" && method === "GET") {
    sendSuccess(response, { devices });
    return true;
  }

  if (
    (url.pathname === "/api/quick-connect/lookup" ||
      url.pathname === "/api/quick-connect/approve") &&
    method === "POST"
  ) {
    const body = await readJSONBody(request);
    const pairing = pendingPairings.get(stringField(body, "code").trim().toUpperCase());

    if (!pairing || pairing.approved) {
      sendFailure(response, 404, INVALID_CODE);
      return true;
    }

    if (url.pathname.endsWith("/lookup")) {
      sendSuccess(response, {
        device_name: pairing.device_name,
        platform: pairing.platform,
        app_version: pairing.app_version,
      });
      return true;
    }

    pairing.approved = true;
    sendSuccess(response, {}, 200, "Device approved. It will finish signing in shortly.");
    return true;
  }

  const deviceMatch = url.pathname.match(/^\/api\/devices\/(\d+)$/);
  if (deviceMatch && (method === "PATCH" || method === "DELETE")) {
    const index = devices.findIndex(device => device.id === Number(deviceMatch[1]));

    if (method === "PATCH") {
      const body = await readJSONBody(request);
      const name = stringField(body, "name").trim();
      if (!name || name.length > 100) {
        sendFailure(response, 400, `name is required and ${NAME_TOO_LONG}`);
        return true;
      }
      if (index === -1) {
        sendFailure(response, 404, "device not found");
        return true;
      }
      devices[index].name = name;
      sendSuccess(response, {}, 200, "Device renamed");
      return true;
    }

    if (index === -1) {
      sendFailure(response, 404, "device not found");
      return true;
    }
    devices.splice(index, 1);
    sendSuccess(response, {}, 200, "Device revoked");
    return true;
  }

  return false;
}

function handleSharedRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  if ((request.method ?? "GET") !== "GET") {
    return false;
  }

  switch (url.pathname) {
    case "/api/notifications/unread-count":
      sendSuccess(response, { unread_count: 0 });
      return true;
    case "/api/music/stats":
      sendSuccess(response, musicStats);
      return true;
    case "/api/tmdb/status":
      sendSuccess(response, { available: true });
      return true;
    default:
      return false;
  }
}

async function handleRequest(request: IncomingMessage, response: ServerResponse) {
  const method = request.method ?? "GET";
  const url = new URL(request.url ?? "/", `http://${HOST}:${PORT}`);

  if (url.pathname === "/health") {
    sendJSON(response, 200, { ok: true });
    return;
  }

  if (
    url.pathname.startsWith("/api/tmdb/images/") ||
    url.pathname.startsWith("/api/static/")
  ) {
    sendPlaceholderImage(response);
    return;
  }

  try {
    if (await handleAuthRoutes(request, response, url)) return;
    if (await handleQuickConnectPublicRoutes(request, response, url)) return;

    const user = currentUser(request);
    if (!user) {
      sendFailure(response, 401, "Unauthorized");
      return;
    }

    if (await handleUserRoutes(request, response, url, user)) return;
    if (url.pathname.startsWith("/api/admin/")) {
      if (!requireAdmin(user, response)) return;
      if (await handleAdminRoutes(request, response, url)) return;
    }
    if (await handleSettingsRoutes(request, response, url, user)) return;
    if (handleMoviesRoutes(request, response, url, user)) return;
    if (handleShowsRoutes(request, response, url)) return;
    if (handleHomeRoutes(request, response, url)) return;
    if (handleSharedRoutes(request, response, url)) return;
    if (await handleDeviceRoutes(request, response, url)) return;

    sendFailure(response, 404, `Unhandled mock API route: ${method} ${url.pathname}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Mock API error";
    sendFailure(response, 500, message);
  }
}

const server = createServer((request, response) => {
  void handleRequest(request, response);
});

server.listen(PORT, HOST, () => {
  process.stdout.write(`E2E mock API listening on http://${HOST}:${PORT}\n`);
});

function closeServer() {
  server.close(() => {
    process.exit(0);
  });
}

process.on("SIGINT", closeServer);
process.on("SIGTERM", closeServer);
