// k6 load scenarios for the Igloo server.
//
// Usage (against a running dev server):
//   k6 run -e SCENARIO=browse -e VUS=3 -e DURATION=5m scripts/perf/soak.js
//   k6 run -e SCENARIO=stream-hls -e MOVIE_ID=1 -e VUS=1 -e DURATION=10m scripts/perf/soak.js
//   k6 run -e SCENARIO=stream-direct -e MOVIE_ID=1 -e VUS=1 -e DURATION=10m scripts/perf/soak.js
//
// Env vars:
//   BASE_URL  (default http://localhost:8080)
//   EMAIL     (default admin@example.com)
//   PASSWORD  (default AdminPassword)
//   SCENARIO  browse | stream-hls | stream-direct  (default browse)
//   MOVIE_ID  required for the stream scenarios
//   PROFILE   HLS profile id (default 720p_3mbps)
//   VUS, DURATION

import http from "k6/http";
import { check, sleep, fail } from "k6";
import { Trend } from "k6/metrics";

const BASE = __ENV.BASE_URL || "http://localhost:8080";
const EMAIL = __ENV.EMAIL || "admin@example.com";
const PASSWORD = __ENV.PASSWORD || "AdminPassword";
const SCENARIO = __ENV.SCENARIO || "browse";
const MOVIE_ID = __ENV.MOVIE_ID || "";
const PROFILE = __ENV.PROFILE || "720p_3mbps";
const VUS = parseInt(__ENV.VUS || "1", 10);
const DURATION = __ENV.DURATION || "5m";

const timeToFirstSegment = new Trend("time_to_first_segment", true);

const execByScenario = {
  browse: "browse",
  "stream-hls": "streamHLS",
  "stream-direct": "streamDirect",
};

export const options = {
  // Keep session cookies across iterations; each VU logs in once.
  noCookiesReset: true,
  scenarios: {
    [SCENARIO]: {
      executor: "constant-vus",
      vus: VUS,
      duration: DURATION,
      exec: execByScenario[SCENARIO],
    },
  },
};

// Session cookie jars are per-VU and persist across iterations, so each VU
// logs in once on its first iteration.
let loggedIn = false;

function ensureLogin() {
  if (loggedIn) {
    return;
  }
  const res = http.post(
    `${BASE}/api/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    { headers: { "Content-Type": "application/json" } },
  );
  if (!check(res, { "login succeeded": (r) => r.status === 200 })) {
    fail(`login failed: ${res.status} ${res.body}`);
  }
  loggedIn = true;
}

function get(path, params) {
  const res = http.get(`${BASE}${path}`, params);
  const ok = check(res, {
    [`GET ${path} ok`]: (r) => r.status >= 200 && r.status < 400,
  });
  if (!ok && __ENV.DEBUG) {
    console.warn(`GET ${path} -> ${res.status} ${String(res.body).slice(0, 200)}`);
  }
  return res;
}

export function browse() {
  ensureLogin();

  get("/api/movies/latest");
  get("/api/movies/continue-watching");
  const lib = get("/api/movies/library?page=1&per_page=24");
  get("/api/music/albums/latest");
  get("/api/music/tracks?page=1&per_page=50");
  get("/api/notifications/unread-count");
  get("/api/search?q=the");

  // Visit a couple of detail pages from the library response when available.
  try {
    const body = JSON.parse(lib.body);
    const movies = (body.data && (body.data.movies || body.data.items)) || [];
    for (const m of movies.slice(0, 2)) {
      if (m && m.id) {
        get(`/api/movies/details/${m.id}`);
        get(`/api/movies/${m.id}/watch-progress`);
      }
    }
  } catch (_) {
    // Envelope shape differs; the library request itself was still measured.
  }

  sleep(1);
}

// The web player identifies its HLS session with a client-generated UUID.
function uuidv4() {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === "x" ? r : (r & 0x3) | 0x8).toString(16);
  });
}

const playbackSession = uuidv4();

export function streamHLS() {
  ensureLogin();
  if (!MOVIE_ID) {
    fail("MOVIE_ID is required for the stream-hls scenario");
  }

  const started = Date.now();
  const manifest = get(
    `/api/movies/${MOVIE_ID}/hls/${PROFILE}/playlist.m3u8?playback_session=${playbackSession}&start=0&audio_track=0`,
  );
  if (manifest.status !== 200) {
    sleep(5);
    return;
  }

  // Segment lines are absolute /api/... URIs with the session query included.
  const lines = String(manifest.body).split("\n");
  const segments = lines.filter((l) => l && !l.startsWith("#"));
  let first = true;
  // Fetch a window of segments, pacing roughly like a player buffering ahead.
  for (const seg of segments.slice(0, 15)) {
    const res = get(seg.trim());
    if (first && res.status === 200) {
      timeToFirstSegment.add(Date.now() - started);
      first = false;
    }
    sleep(first ? 0 : 2);
  }
}

export function streamDirect() {
  ensureLogin();
  if (!MOVIE_ID) {
    fail("MOVIE_ID is required for the stream-direct scenario");
  }

  const chunk = 8 * 1024 * 1024;
  // Read sequential ranges like a player pulling a direct-play file.
  for (let i = 0; i < 10; i++) {
    const start = i * chunk;
    const res = http.get(`${BASE}/api/movies/${MOVIE_ID}/stream`, {
      headers: { Range: `bytes=${start}-${start + chunk - 1}` },
    });
    check(res, { "range request ok": (r) => r.status === 206 || r.status === 200 });
    if (res.status !== 206 && res.status !== 200) {
      break;
    }
    sleep(2);
  }
}

export function teardown() {
  // Best effort: stop any personal HLS session this run started. teardown()
  // runs in a fresh VU, so it needs its own login.
  if (SCENARIO === "stream-hls" && MOVIE_ID) {
    http.post(
      `${BASE}/api/auth/login`,
      JSON.stringify({ email: EMAIL, password: PASSWORD }),
      { headers: { "Content-Type": "application/json" } },
    );
    http.post(
      `${BASE}/api/movies/${MOVIE_ID}/hls/session/stop?playback_session=${playbackSession}`,
    );
  }
}
