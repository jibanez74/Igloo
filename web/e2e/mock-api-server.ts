import { randomUUID } from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import {
  MOVIES_PER_PAGE,
} from "../src/lib/constants";

type ApiData = Record<string, unknown>;

type User = {
  id: number;
  name: string;
  email: string;
  password: string;
  is_admin: boolean;
  avatar: string | null;
  pin: string | null;
  created_at: string;
  updated_at: string;
};

type LibrarySettings = {
  movies_dir: string | null;
  shows_dir: string | null;
  music_dir: string | null;
};

type GeneralSettings = {
  tmdb_key: string | null;
  immich_base_url: string | null;
  immich_api_key: string | null;
  jellyfin_base_url: string | null;
  jellyfin_api_key: string | null;
  spotify_client_id: string | null;
  spotify_client_secret: string | null;
  enable_watcher: boolean;
  download_images: boolean;
  static_dir: string;
  transcode_dir: string;
};

// Global settings served by the playback endpoint; writes are admin-only.
type ServerPlaybackSettings = {
  server_upload_mbps: number | null;
  hardware_acceleration_device: "cpu" | "apple" | "nvidia" | "intel";
};

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

type MovieWatchProgress = {
  progress_sec: number | null;
  duration_sec: number | null;
  watched: boolean;
  updated_at: string | null;
};

type MockDevice = {
  id: number;
  name: string;
  platform: string;
  app_version: string | null;
  created_at: string;
  last_used_at: string;
};

type PendingPairing = {
  secret: string;
  device_name: string;
  platform: string;
  app_version: string | null;
  approved: boolean;
};

type SortDirection = "asc" | "desc";

const HOST = "127.0.0.1";
const PORT = Number.parseInt(process.env.E2E_MOCK_API_PORT ?? "8080", 10);
const SESSION_COOKIE = "igloo_e2e_session";
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin@example.com";
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword";
const startedAt = new Date().toISOString();

let nextUserId = 2;
let nextDeviceId = 1;

const sessions = new Map<string, number>();
// Quick Connect pairing state. Starts empty so specs that only render the
// settings page see no devices. Bearer-token auth on other routes is
// deliberately not simulated: token validity/revocation semantics are covered
// by the Go integration tests and the live-gated quick-connect.spec.ts.
const devices: MockDevice[] = [];
const pendingPairings = new Map<string, PendingPairing>();
const likedMovieIds = new Set<number>();
const watchProgress = new Map<number, MovieWatchProgress>();
const moviePlaylists = [
  {
    id: 1,
    user_id: 1,
    name: "Weekend queue",
    description: nullableString("A few movies for E2E navigation."),
    cover_image: nullableString("/signal-fire.jpg"),
    is_public: false,
    movie_id: nullableInt(null),
    content_type: "movie",
    created_at: startedAt,
    updated_at: startedAt,
    movie_count: 1,
    is_owner: true,
    can_edit: true,
  },
];

const users: User[] = [
  {
    id: 1,
    name: "Igloo Admin",
    email: ADMIN_EMAIL,
    password: ADMIN_PASSWORD,
    is_admin: true,
    avatar: null,
    pin: null,
    created_at: startedAt,
    updated_at: startedAt,
  },
];

let librarySettings: LibrarySettings = {
  movies_dir: "/srv/media/movies",
  shows_dir: null,
  music_dir: "/srv/media/music",
};

let generalSettings: GeneralSettings = {
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

const playbackProfiles = [
  { id: "2160p_16mbps", label: "4K · 16 Mbps", height: 2160, video_mbps: 16 },
  { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
  { id: "1080p_6mbps", label: "1080p · 6 Mbps", height: 1080, video_mbps: 6 },
  { id: "1080p_4mbps", label: "1080p · 4 Mbps", height: 1080, video_mbps: 4 },
  { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
];

function nullableString(value: string | null) {
  return { String: value ?? "", Valid: value !== null && value !== "" };
}

function nullableInt(value: number | null) {
  return { Int64: value ?? 0, Valid: value !== null };
}

function nullableFloat(value: number | null) {
  return { Float64: value ?? 0, Valid: value !== null };
}

const libraryMovies = [
  {
    id: 101,
    title: "Signal Fire",
    poster_path: nullableString("/signal-fire.jpg"),
    year: nullableInt(2024),
    certification: nullableString("PG-13"),
  },
  {
    id: 102,
    title: "Northern Relay",
    poster_path: nullableString("/northern-relay.jpg"),
    year: nullableInt(2023),
    certification: nullableString("PG"),
  },
  {
    id: 103,
    title: "Harbor Lights",
    poster_path: nullableString("/harbor-lights.jpg"),
    year: nullableInt(2022),
    certification: nullableString("R"),
  },
];

const latestAlbums = [
  {
    id: 201,
    title: "Warm Static",
    cover: nullableString("/api/static/albums/warm-static.svg"),
    musician: nullableString("The Signals"),
    year: nullableInt(2024),
  },
  {
    id: 202,
    title: "Night Index",
    cover: nullableString("/api/static/albums/night-index.svg"),
    musician: nullableString("June Harbor"),
    year: nullableInt(2023),
  },
];

const musicians = [
  {
    id: 301,
    name: "The Signals",
    sort_name: "Signals, The",
    thumb: nullableString("/api/static/musicians/the-signals.svg"),
    album_count: 1,
    track_count: 2,
  },
  {
    id: 302,
    name: "June Harbor",
    sort_name: "Harbor, June",
    thumb: nullableString("/api/static/musicians/june-harbor.svg"),
    album_count: 1,
    track_count: 1,
  },
];

const tracks = [
  {
    id: 401,
    title: "Beacon Line",
    duration: 214_000,
    codec: "flac",
    bit_rate: 890000,
    file_path: "/srv/media/music/beacon-line.flac",
    album_id: nullableInt(201),
    album_title: nullableString("Warm Static"),
    album_cover: nullableString("/api/static/albums/warm-static.svg"),
    musician_id: nullableInt(301),
    musician_name: nullableString("The Signals"),
  },
  {
    id: 402,
    title: "Soft Cutoff",
    duration: 188_000,
    codec: "flac",
    bit_rate: 820000,
    file_path: "/srv/media/music/soft-cutoff.flac",
    album_id: nullableInt(201),
    album_title: nullableString("Warm Static"),
    album_cover: nullableString("/api/static/albums/warm-static.svg"),
    musician_id: nullableInt(301),
    musician_name: nullableString("The Signals"),
  },
  {
    id: 403,
    title: "Late Platform",
    duration: 236_000,
    codec: "aac",
    bit_rate: 256000,
    file_path: "/srv/media/music/late-platform.m4a",
    album_id: nullableInt(202),
    album_title: nullableString("Night Index"),
    album_cover: nullableString("/api/static/albums/night-index.svg"),
    musician_id: nullableInt(302),
    musician_name: nullableString("June Harbor"),
  },
];

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

function apiSuccess(data: ApiData = {}, message?: string) {
  return { error: false, ...(message ? { message } : {}), data };
}

function apiFailure(message: string) {
  return { error: true, message };
}

function sendJSON(
  response: ServerResponse,
  status: number,
  body: Record<string, unknown>,
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
  data: ApiData = {},
  status = 200,
  message?: string,
  headers?: Record<string, string>,
) {
  sendJSON(response, status, apiSuccess(data, message), headers);
}

function sendFailure(response: ServerResponse, status: number, message: string) {
  sendJSON(response, status, apiFailure(message));
}

function sendNoContent(response: ServerResponse) {
  response.writeHead(204, { "Cache-Control": "no-store" });
  response.end();
}

function sendPlaceholderImage(response: ServerResponse, label: string) {
  const safeLabel = label.replace(/[<>&]/g, "");
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="960" viewBox="0 0 640 960"><rect width="640" height="960" fill="#0f172a"/><rect x="48" y="48" width="544" height="864" rx="24" fill="#1e293b"/><text x="320" y="480" fill="#f59e0b" font-family="Arial, sans-serif" font-size="42" font-weight="700" text-anchor="middle">${safeLabel}</text></svg>`;
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

async function readRequestBody(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  return Buffer.concat(chunks).toString("utf8");
}

async function readJSONBody(request: IncomingMessage) {
  const raw = await readRequestBody(request);
  if (!raw) return {};

  const parsed = JSON.parse(raw) as unknown;
  return asRecord(parsed) ?? {};
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
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

function nullableStringField(body: Record<string, unknown>, key: string) {
  const value = body[key];
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed === "" ? null : trimmed;
  }
  return value === null ? null : undefined;
}

function nullableNumberField(body: Record<string, unknown>, key: string) {
  const value = body[key];
  if (typeof value === "number" && Number.isFinite(value)) return value;
  return value === null ? null : undefined;
}

function valueOrCurrent<T>(value: T | undefined, current: T) {
  return value === undefined ? current : value;
}

function publicUser(user: User) {
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    is_admin: user.is_admin,
    avatar: user.avatar,
    has_pin: user.pin !== null,
    created_at: user.created_at,
    updated_at: user.updated_at,
  };
}

function requireAuth(request: IncomingMessage, response: ServerResponse) {
  const user = currentUser(request);
  if (!user) {
    sendFailure(response, 401, "Unauthorized");
    return null;
  }
  return user;
}

function requireAdmin(request: IncomingMessage, response: ServerResponse) {
  const user = requireAuth(request, response);
  if (!user) return null;
  if (!user.is_admin) {
    sendFailure(response, 403, "Admin privileges required.");
    return null;
  }
  return user;
}

function findUserByEmail(email: string) {
  const normalizedEmail = email.trim().toLowerCase();
  return users.find(user => user.email.toLowerCase() === normalizedEmail) ?? null;
}

function touchUser(user: User) {
  user.updated_at = new Date().toISOString();
}

function removeSessionsForUser(userId: number) {
  for (const [sessionId, sessionUserId] of sessions) {
    if (sessionUserId === userId) {
      sessions.delete(sessionId);
    }
  }
}

function paginationParams(url: URL) {
  const page = Math.max(1, Number.parseInt(url.searchParams.get("page") ?? "1", 10));
  const perPage = Math.max(
    1,
    Number.parseInt(
      url.searchParams.get("per_page") ?? String(MOVIES_PER_PAGE),
      10,
    ),
  );
  const sort: SortDirection =
    url.searchParams.get("sort") === "desc" ? "desc" : "asc";
  return { page, perPage, sort };
}

function paginate<T>(items: T[], page: number, perPage: number) {
  const start = (page - 1) * perPage;
  return {
    items: items.slice(start, start + perPage),
    total: items.length,
    total_pages: Math.max(1, Math.ceil(items.length / perPage)),
  };
}

function sortedMovies(sort: SortDirection) {
  return [...libraryMovies].sort((a, b) => {
    const value = a.title.localeCompare(b.title);
    return sort === "asc" ? value : -value;
  });
}

function movieDetails(id: number) {
  const baseMovie = libraryMovies.find(movie => movie.id === id) ?? libraryMovies[0];

  return {
    movie: {
      id: baseMovie.id,
      title: baseMovie.title,
      file_path: `/srv/media/movies/${baseMovie.title}.mp4`,
      file_name: `${baseMovie.title}.mp4`,
      size: 4_200_000_000,
      container: "mp4",
      mime_type: "video/mp4",
      adult: false,
      tmdb_id: nullableInt(900000 + baseMovie.id),
      imdb_id: nullableString(`tt${900000 + baseMovie.id}`),
      poster_path: baseMovie.poster_path,
      backdrop_path: nullableString(`/${baseMovie.title.toLowerCase().replaceAll(" ", "-")}-backdrop.jpg`),
      language: nullableString("en"),
      year: baseMovie.year,
      release_date: nullableString("2024-04-12"),
      overview: nullableString(
        `${baseMovie.title} is a compact mock movie used by the E2E harness.`,
      ),
      tag_line: nullableString("Small server, big library."),
      certification: baseMovie.certification,
      critic_rating: nullableFloat(7.8),
      audience_rating: nullableFloat(8.1),
      revenue: nullableFloat(1_200_000),
      budget: nullableFloat(750_000),
      run_time: nullableInt(122),
      duration: nullableFloat(7320),
      created_at: startedAt,
      updated_at: startedAt,
    },
    cast: [
      {
        id: 1,
        movie_id: baseMovie.id,
        artist_id: 11,
        character: "Mara Voss",
        cast_order: 0,
        artist_name: "Alex Vega",
        artist_profile: nullableString("/alex-vega.jpg"),
      },
      {
        id: 2,
        movie_id: baseMovie.id,
        artist_id: 12,
        character: "Eli Storm",
        cast_order: 1,
        artist_name: "Sam Rivera",
        artist_profile: nullableString("/sam-rivera.jpg"),
      },
    ],
    crew: [
      {
        id: 1,
        movie_id: baseMovie.id,
        artist_id: 21,
        job: "Director",
        department: "Directing",
        artist_name: "Nora Finch",
        artist_profile: nullableString("/nora-finch.jpg"),
      },
      {
        id: 2,
        movie_id: baseMovie.id,
        artist_id: 22,
        job: "Writer",
        department: "Writing",
        artist_name: "Ira Chen",
        artist_profile: nullableString("/ira-chen.jpg"),
      },
    ],
    genres: [
      { id: 10, tag: "Drama" },
      { id: 20, tag: "Adventure" },
    ],
    production_companies: [
      {
        id: 1,
        name: "Igloo Pictures",
        tmdb_id: 7001,
        logo: nullableString("/igloo-pictures.svg"),
        country: nullableString("US"),
      },
    ],
    extra_videos: [
      {
        id: 1,
        title: "Official Trailer",
        external_id: nullableString("dQw4w9WgXcQ"),
        key: "dQw4w9WgXcQ",
        type: "Trailer",
        site: "YouTube",
        official: true,
        created_at: startedAt,
        updated_at: startedAt,
      },
    ],
  };
}

function movieTechnicalDetails(id: number) {
  const details = movieDetails(id);
  return {
    movie: {
      file_name: details.movie.file_name,
      file_path: details.movie.file_path,
      size: details.movie.size,
      container: details.movie.container,
      mime_type: details.movie.mime_type,
      run_time: details.movie.run_time,
      duration: details.movie.duration,
    },
    video_streams: [
      {
        id: 1,
        movie_id: id,
        stream_index: 0,
        codec: "h264",
        codec_profile: nullableString("High"),
        codec_level: nullableInt(41),
        bit_rate: 8_000_000,
        width: 1920,
        height: 1080,
        coded_width: nullableInt(1920),
        coded_height: nullableInt(1080),
        aspect_ratio: nullableString("16:9"),
        frame_rate: 23.976,
        avg_frame_rate: nullableString("24000/1001"),
        bit_depth: nullableInt(8),
        pixel_format: nullableString("yuv420p"),
        color_range: nullableString("tv"),
        color_space: nullableString("bt709"),
        color_primaries: nullableString("bt709"),
        color_transfer: nullableString("bt709"),
        field_order: nullableString("progressive"),
        rotation: nullableInt(null),
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
        sample_rate: nullableInt(48000),
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
        movie_id: nullableInt(id),
      },
      {
        id: 2,
        title: "The Journey",
        start_time: 372,
        thumb: nullableString(null),
        movie_id: nullableInt(id),
      },
    ],
  };
}



function playbackSettingsResponse() {
  return {
    profiles: playbackProfiles,
    server_upload_mbps: serverPlaybackSettings.server_upload_mbps,
    hardware_acceleration_device:
      serverPlaybackSettings.hardware_acceleration_device,
  };
}

function canHandleWithoutAuth(pathname: string) {
  return (
    pathname === "/health" ||
    pathname === "/api/auth/login" ||
    pathname === "/api/auth/logout" ||
    pathname === "/api/auth/user" ||
    pathname === "/api/quick-connect/initiate" ||
    pathname === "/api/quick-connect/redeem" ||
    pathname.startsWith("/api/tmdb/images/") ||
    pathname.startsWith("/api/youtube/thumbnails/") ||
    pathname.startsWith("/api/static/")
  );
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
      sendFailure(
        response,
        400,
        "device_name is required and must be at most 100 characters",
      );
      return true;
    }

    let code = "";
    do {
      code = randomUUID().replace(/[^A-Z2-9]/gi, "").toUpperCase().slice(0, 6);
    } while (code.length < 6 || pendingPairings.has(code));

    pendingPairings.set(code, {
      secret: randomUUID(),
      device_name: deviceName,
      platform: stringField(body, "platform"),
      app_version: nullableStringField(body, "app_version") ?? null,
      approved: false,
    });

    sendSuccess(
      response,
      {
        code,
        secret: pendingPairings.get(code)?.secret,
        expires_in_seconds: 300,
        poll_interval_seconds: 2,
      },
      201,
    );
    return true;
  }

  if (url.pathname === "/api/quick-connect/redeem" && method === "POST") {
    const body = await readJSONBody(request);
    const code = stringField(body, "code").trim().toUpperCase();
    const pairing = pendingPairings.get(code);

    if (!pairing || pairing.secret !== stringField(body, "secret")) {
      sendFailure(response, 404, "invalid or expired code");
      return true;
    }

    if (!pairing.approved) {
      sendSuccess(response, { status: "pending" });
      return true;
    }

    const now = new Date().toISOString();
    const device: MockDevice = {
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
      device: { ...device, is_current: true },
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
    const email = stringField(body, "email").trim();
    const password = stringField(body, "password");
    const user = findUserByEmail(email);

    if (!user || user.password !== password) {
      sendFailure(response, 401, "Invalid email or password.");
      return true;
    }

    const sessionId = randomUUID();
    sessions.set(sessionId, user.id);
    sendSuccess(
      response,
      { user: publicUser(user) },
      200,
      "Login successful",
      {
        "Set-Cookie": `${SESSION_COOKIE}=${encodeURIComponent(sessionId)}; Path=/; HttpOnly; SameSite=Lax`,
      },
    );
    return true;
  }

  if (url.pathname === "/api/auth/logout" && method === "DELETE") {
    const sessionId = parseCookies(request).get(SESSION_COOKIE);
    if (sessionId) {
      sessions.delete(sessionId);
    }
    sendSuccess(response, {}, 200, "Logged out", {
      "Set-Cookie": `${SESSION_COOKIE}=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax`,
    });
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
      sendFailure(response, 409, "A user with that email already exists.");
      return true;
    }
    user.email = email;
    touchUser(user);
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  if (url.pathname === "/api/user/password" && method === "PUT") {
    const body = await readJSONBody(request);
    const currentPassword = stringField(body, "current_password");
    const newPassword = stringField(body, "new_password");
    if (user.password !== currentPassword) {
      sendFailure(response, 400, "Current password is incorrect.");
      return true;
    }
    user.password = newPassword;
    touchUser(user);
    sendSuccess(response, {});
    return true;
  }

  if (url.pathname === "/api/user/pin" && method === "GET") {
    sendSuccess(response, { pin: user.pin });
    return true;
  }

  if (url.pathname === "/api/user/pin" && method === "PUT") {
    const body = await readJSONBody(request);
    const pin = stringField(body, "pin");
    const currentPin = stringField(body, "current_pin");
    if (pin !== "" && !/^\d{4}$/.test(pin)) {
      sendFailure(response, 400, "pin must be exactly 4 digits");
      return true;
    }
    if (user.pin !== null) {
      if (!currentPin) {
        sendFailure(response, 400, "current PIN is required");
        return true;
      }
      if (currentPin !== user.pin) {
        sendFailure(response, 401, "current PIN is incorrect");
        return true;
      }
    }
    user.pin = pin === "" ? null : pin;
    touchUser(user);
    sendSuccess(
      response,
      { user: publicUser(user) },
      200,
      pin === "" ? "PIN removed successfully" : "PIN updated successfully",
    );
    return true;
  }

  if (url.pathname === "/api/user/avatar" && method === "PUT") {
    const body = await readJSONBody(request);
    user.avatar = stringField(body, "avatar", user.avatar ?? "").trim() || null;
    touchUser(user);
    sendSuccess(response, { user: publicUser(user) });
    return true;
  }

  if (url.pathname === "/api/user" && method === "DELETE") {
    const index = users.findIndex(item => item.id === user.id);
    if (index >= 0) {
      users.splice(index, 1);
      removeSessionsForUser(user.id);
    }
    sendSuccess(response, {}, 200, "Account deleted", {
      "Set-Cookie": `${SESSION_COOKIE}=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax`,
    });
    return true;
  }

  return false;
}

async function handleAdminRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
  admin: User,
) {
  const method = request.method ?? "GET";
  if (admin.id <= 0) return false;

  if (url.pathname === "/api/admin/users" && method === "GET") {
    sendSuccess(response, { users: users.map(publicUser) });
    return true;
  }

  if (url.pathname === "/api/admin/users" && method === "POST") {
    const body = await readJSONBody(request);
    const email = stringField(body, "email").trim();
    if (findUserByEmail(email)) {
      sendFailure(response, 409, "A user with that email already exists.");
      return true;
    }
    const now = new Date().toISOString();
    const user: User = {
      id: nextUserId,
      name: stringField(body, "name", "New User").trim(),
      email,
      password: stringField(body, "password"),
      is_admin: booleanField(body, "is_admin"),
      avatar: null,
      pin: null,
      created_at: now,
      updated_at: now,
    };
    nextUserId += 1;
    users.push(user);
    sendSuccess(response, { user: publicUser(user) }, 201);
    return true;
  }

  const passwordMatch = url.pathname.match(/^\/api\/admin\/users\/(\d+)\/password$/);
  if (passwordMatch && method === "PUT") {
    const target = users.find(item => item.id === Number(passwordMatch[1]));
    if (!target) {
      sendFailure(response, 404, "User not found.");
      return true;
    }
    const body = await readJSONBody(request);
    target.password = stringField(body, "password", target.password);
    touchUser(target);
    sendSuccess(response, {});
    return true;
  }

  const userMatch = url.pathname.match(/^\/api\/admin\/users\/(\d+)$/);
  if (userMatch && method === "PATCH") {
    const target = users.find(item => item.id === Number(userMatch[1]));
    if (!target) {
      sendFailure(response, 404, "User not found.");
      return true;
    }
    const body = await readJSONBody(request);
    const email = stringField(body, "email", target.email).trim();
    const duplicate = findUserByEmail(email);
    if (duplicate && duplicate.id !== target.id) {
      sendFailure(response, 409, "A user with that email already exists.");
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
    const userId = Number(userMatch[1]);
    const index = users.findIndex(item => item.id === userId);
    if (index >= 0) {
      users.splice(index, 1);
      removeSessionsForUser(userId);
    }
    sendSuccess(response, {});
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

  if (url.pathname === "/api/settings" && method === "GET") {
    sendSuccess(response, librarySettings);
    return true;
  }

  if (url.pathname === "/api/settings/libraries" && method === "PUT") {
    if (!user.is_admin) {
      sendFailure(response, 403, "Admin privileges required.");
      return true;
    }
    const body = await readJSONBody(request);
    librarySettings = {
      movies_dir: valueOrCurrent(
        nullableStringField(body, "movies_dir"),
        librarySettings.movies_dir,
      ),
      shows_dir: valueOrCurrent(
        nullableStringField(body, "shows_dir"),
        librarySettings.shows_dir,
      ),
      music_dir: valueOrCurrent(
        nullableStringField(body, "music_dir"),
        librarySettings.music_dir,
      ),
    };
    sendSuccess(response, { settings: librarySettings });
    return true;
  }

  if (url.pathname === "/api/settings/general" && method === "GET") {
    if (!user.is_admin) {
      sendFailure(response, 403, "Admin privileges required.");
      return true;
    }
    sendSuccess(response, { settings: generalSettings });
    return true;
  }

  if (url.pathname === "/api/settings/general" && method === "PUT") {
    if (!user.is_admin) {
      sendFailure(response, 403, "Admin privileges required.");
      return true;
    }
    const body = await readJSONBody(request);
    generalSettings = {
      tmdb_key: valueOrCurrent(
        nullableStringField(body, "tmdb_key"),
        generalSettings.tmdb_key,
      ),
      immich_base_url: valueOrCurrent(
        nullableStringField(body, "immich_base_url"),
        generalSettings.immich_base_url,
      ),
      immich_api_key: valueOrCurrent(
        nullableStringField(body, "immich_api_key"),
        generalSettings.immich_api_key,
      ),
      jellyfin_base_url: valueOrCurrent(
        nullableStringField(body, "jellyfin_base_url"),
        generalSettings.jellyfin_base_url,
      ),
      jellyfin_api_key: valueOrCurrent(
        nullableStringField(body, "jellyfin_api_key"),
        generalSettings.jellyfin_api_key,
      ),
      spotify_client_id: valueOrCurrent(
        nullableStringField(body, "spotify_client_id"),
        generalSettings.spotify_client_id,
      ),
      spotify_client_secret: valueOrCurrent(
        nullableStringField(body, "spotify_client_secret"),
        generalSettings.spotify_client_secret,
      ),
      enable_watcher: booleanField(
        body,
        "enable_watcher",
        generalSettings.enable_watcher,
      ),
      download_images: booleanField(
        body,
        "download_images",
        generalSettings.download_images,
      ),
      static_dir: stringField(body, "static_dir", generalSettings.static_dir),
      transcode_dir: stringField(body, "transcode_dir", generalSettings.transcode_dir),
    };
    sendSuccess(response, { settings: generalSettings, restart_required: false });
    return true;
  }

  if (url.pathname === "/api/settings/playback" && method === "GET") {
    sendSuccess(response, { settings: playbackSettingsResponse() });
    return true;
  }

  if (url.pathname === "/api/settings/playback" && method === "PUT") {
    // Mirrors the RequireAdmin middleware on the real route.
    if (!user.is_admin) {
      sendFailure(response, 403, "Server playback settings are admin-only.");
      return true;
    }

    const body = await readJSONBody(request);

    // Absent fields keep their current value; the ones that were sent are
    // validated exactly as the Go handler validates them.
    let serverUploadMbps = serverPlaybackSettings.server_upload_mbps;
    if (Object.prototype.hasOwnProperty.call(body, "server_upload_mbps")) {
      const sent = nullableNumberField(body, "server_upload_mbps");
      if (
        typeof sent === "number" &&
        (sent <= 0 || sent >= SERVER_UPLOAD_MAX_MBPS)
      ) {
        sendFailure(
          response,
          400,
          `server upload speed must be greater than 0 and less than ${SERVER_UPLOAD_MAX_MBPS} Mbps`,
        );
        return true;
      }
      serverUploadMbps = valueOrCurrent(sent, serverUploadMbps);
    }

    let hardwareDevice = serverPlaybackSettings.hardware_acceleration_device;
    if (
      Object.prototype.hasOwnProperty.call(body, "hardware_acceleration_device")
    ) {
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

function handleMoviesRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/movies/latest" && method === "GET") {
    sendSuccess(response, { movies: libraryMovies.slice(0, 3) });
    return true;
  }

  if (url.pathname === "/api/movies/continue-watching" && method === "GET") {
    sendSuccess(response, {
      movies: libraryMovies.slice(0, 2).map((movie, index) => ({
        ...movie,
        progress_sec: 900 * (index + 1),
        duration_sec: 5400,
      })),
    });
    return true;
  }

  if (url.pathname === "/api/movies/stats" && method === "GET") {
    sendSuccess(response, { total_movies: libraryMovies.length });
    return true;
  }

  if (url.pathname === "/api/movies/library" && method === "GET") {
    const { page, perPage, sort } = paginationParams(url);
    const movies = sortedMovies(sort);
    const { items, total, total_pages } = paginate(movies, page, perPage);
    sendSuccess(response, {
      movies: items,
      total,
      page,
      per_page: perPage,
      total_pages,
      sort,
    });
    return true;
  }

  if (url.pathname === "/api/movies/genres" && method === "GET") {
    sendSuccess(response, {
      genres: [
        { genre_id: 10, genre_tag: "Drama", movie_count: 2 },
        { genre_id: 20, genre_tag: "Adventure", movie_count: 1 },
      ],
    });
    return true;
  }

  if (url.pathname === "/api/movies/playlists" && method === "GET") {
    sendSuccess(response, { playlists: moviePlaylists });
    return true;
  }

  if (url.pathname === "/api/movies/liked" && method === "GET") {
    const { page, perPage, sort } = paginationParams(url);
    const movies = sortedMovies(sort).filter(movie => likedMovieIds.has(movie.id));
    const { items, total, total_pages } = paginate(movies, page, perPage);
    sendSuccess(response, {
      movies: items,
      total,
      page,
      per_page: perPage,
      total_pages,
      sort,
    });
    return true;
  }

  const detailsMatch = url.pathname.match(/^\/api\/movies\/details\/(\d+)$/);
  if (detailsMatch && method === "GET") {
    sendSuccess(response, movieDetails(Number(detailsMatch[1])));
    return true;
  }

  const technicalMatch = url.pathname.match(
    /^\/api\/movies\/(\d+)\/technical-details$/,
  );
  if (technicalMatch && method === "GET") {
    sendSuccess(response, movieTechnicalDetails(Number(technicalMatch[1])));
    return true;
  }

  const progressMatch = url.pathname.match(/^\/api\/movies\/(\d+)\/watch-progress$/);
  if (progressMatch && method === "GET") {
    const movieId = Number(progressMatch[1]);
    sendSuccess(
      response,
      watchProgress.get(movieId) ?? {
        progress_sec: null,
        duration_sec: null,
        watched: false,
        updated_at: null,
      },
    );
    return true;
  }

  if (progressMatch && method === "PUT") {
    const movieId = Number(progressMatch[1]);
    watchProgress.set(movieId, {
      progress_sec: 0,
      duration_sec: 7320,
      watched: false,
      updated_at: new Date().toISOString(),
    });
    sendSuccess(response, { watched: false });
    return true;
  }

  if (progressMatch && method === "DELETE") {
    watchProgress.delete(Number(progressMatch[1]));
    sendSuccess(response, { cleared: true });
    return true;
  }

  const streamMatch = url.pathname.match(/^\/api\/movies\/(\d+)\/stream$/);
  if (streamMatch && method === "GET") {
    sendNoContent(response);
    return true;
  }

  return false;
}

function handleMusicRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/music/stats" && method === "GET") {
    sendSuccess(response, {
      total_albums: latestAlbums.length,
      total_tracks: tracks.length,
      total_musicians: musicians.length,
    });
    return true;
  }

  if (url.pathname === "/api/music/albums/latest" && method === "GET") {
    sendSuccess(response, { albums: latestAlbums });
    return true;
  }

  return false;
}

function handleTmdbRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/tmdb/status" && method === "GET") {
    sendSuccess(response, { available: true });
    return true;
  }

  if (url.pathname === "/api/tmdb/movies/in-theaters" && method === "GET") {
    sendSuccess(response, { movies: theaterMovies });
    return true;
  }

  return false;
}

function handleWatchRoomRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/watch-rooms" && method === "GET") {
    sendSuccess(response, { rooms: [] });
    return true;
  }

  return false;
}

async function handleDeviceRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/devices" && method === "GET") {
    sendSuccess(response, {
      devices: devices.map(device => ({ ...device, is_current: false })),
    });
    return true;
  }

  if (url.pathname === "/api/quick-connect/lookup" && method === "POST") {
    const body = await readJSONBody(request);
    const code = stringField(body, "code").trim().toUpperCase();
    const pairing = pendingPairings.get(code);

    if (!pairing || pairing.approved) {
      sendFailure(response, 404, "invalid or expired code");
      return true;
    }

    sendSuccess(response, {
      device_name: pairing.device_name,
      platform: pairing.platform,
      app_version: pairing.app_version,
    });
    return true;
  }

  if (url.pathname === "/api/quick-connect/approve" && method === "POST") {
    const body = await readJSONBody(request);
    const code = stringField(body, "code").trim().toUpperCase();
    const pairing = pendingPairings.get(code);

    if (!pairing || pairing.approved) {
      sendFailure(response, 404, "invalid or expired code");
      return true;
    }

    pairing.approved = true;
    sendSuccess(response, {}, 200, "Device approved. It will finish signing in shortly.");
    return true;
  }

  const deviceMatch = url.pathname.match(/^\/api\/devices\/(\d+)$/);
  if (deviceMatch) {
    const deviceId = Number.parseInt(deviceMatch[1], 10);
    const index = devices.findIndex(device => device.id === deviceId);

    if (method === "PATCH") {
      const body = await readJSONBody(request);
      const name = stringField(body, "name").trim();
      if (!name || name.length > 100) {
        sendFailure(
          response,
          400,
          "name is required and must be at most 100 characters",
        );
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

    if (method === "DELETE") {
      if (index === -1) {
        sendFailure(response, 404, "device not found");
        return true;
      }
      devices.splice(index, 1);
      sendSuccess(response, {}, 200, "Device revoked");
      return true;
    }
  }

  return false;
}

function handleNotificationRoutes(
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
) {
  const method = request.method ?? "GET";

  if (url.pathname === "/api/notifications/unread-count" && method === "GET") {
    sendSuccess(response, { unread_count: 0 });
    return true;
  }

  return false;
}

async function handleRequest(request: IncomingMessage, response: ServerResponse) {
  const method = request.method ?? "GET";
  const url = new URL(request.url ?? "/", `http://${HOST}:${PORT}`);

  if (method === "OPTIONS") {
    response.writeHead(204, {
      "Access-Control-Allow-Methods": "GET,POST,PUT,PATCH,DELETE,OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    });
    response.end();
    return;
  }

  if (url.pathname === "/health") {
    sendJSON(response, 200, { ok: true });
    return;
  }

  if (url.pathname.startsWith("/api/tmdb/images/")) {
    sendPlaceholderImage(response, "Igloo");
    return;
  }

  if (url.pathname.startsWith("/api/youtube/thumbnails/")) {
    sendPlaceholderImage(response, "Igloo");
    return;
  }

  if (url.pathname.startsWith("/api/static/")) {
    sendPlaceholderImage(response, "Igloo");
    return;
  }

  try {
    if (await handleAuthRoutes(request, response, url)) return;
    if (await handleQuickConnectPublicRoutes(request, response, url)) return;

    if (!canHandleWithoutAuth(url.pathname)) {
      const user = requireAuth(request, response);
      if (!user) return;

      if (await handleUserRoutes(request, response, url, user)) return;

      if (url.pathname.startsWith("/api/admin/")) {
        const admin = requireAdmin(request, response);
        if (!admin) return;
        if (await handleAdminRoutes(request, response, url, admin)) return;
      }

      if (await handleSettingsRoutes(request, response, url, user)) return;
      if (handleMoviesRoutes(request, response, url)) return;
      if (handleMusicRoutes(request, response, url)) return;
      if (handleTmdbRoutes(request, response, url)) return;
      if (handleWatchRoomRoutes(request, response, url)) return;
      if (handleNotificationRoutes(request, response, url)) return;
      if (await handleDeviceRoutes(request, response, url)) return;
    }

    if (url.pathname.startsWith("/api/")) {
      sendFailure(response, 404, `Unhandled mock API route: ${method} ${url.pathname}`);
      return;
    }

    sendFailure(response, 404, "Not found");
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
