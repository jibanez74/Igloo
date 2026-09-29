import { APP_NAME } from "@/lib/constants";

// Search results and link previews truncate around this length.
const DESCRIPTION_MAX_LENGTH = 160;

/**
 * The `head` value for a route: `<title>` reads "<title> - Igloo", and the
 * description meta is set only when there is one, so a route without its own
 * keeps the parent's (ultimately the root default). Build static heads once at
 * module level and return the constant from `head`.
 */
export function routeHead(title: string, description?: string | null) {
  return {
    meta: [
      { title: `${title} - ${APP_NAME}` },
      ...(description
        ? [
            {
              name: "description",
              content: description.slice(0, DESCRIPTION_MAX_LENGTH),
            },
          ]
        : []),
    ],
  };
}

/** The fields a movie or show detail route's loader hands its `head`. */
export type MediaHeadData = {
  title: string;
  year: number | null;
  overview: string | null;
};

type MediaHead = ReturnType<typeof routeHead>;

// "<Title> (<year>)", described by the overview or a stock sentence.
function mediaHead(
  media: MediaHeadData | null | undefined,
  fallback: MediaHead,
  verb: string,
) {
  if (!media) return fallback;

  return routeHead(
    media.year ? `${media.title} (${media.year})` : media.title,
    media.overview || `${verb} ${media.title} in your ${APP_NAME} media library.`,
  );
}

/**
 * Head for a music detail page: "Listen to <title> - <summary> in your Igloo
 * <place>.", where the summary carries the page's counts or duration.
 */
export function listenHead(
  title: string,
  summary: string,
  place = "music library",
) {
  return routeHead(
    title,
    `Listen to ${title} - ${summary} in your ${APP_NAME} ${place}.`,
  );
}

const MOVIE_FALLBACK_HEAD = routeHead("Movie");
const SHOW_FALLBACK_HEAD = routeHead("TV Show");

/** Head for the library and In Theaters movie detail pages. */
export function movieHead(movie: MediaHeadData | null | undefined) {
  return mediaHead(movie, MOVIE_FALLBACK_HEAD, "Watch");
}

/** Head for the TV show detail page. */
export function showHead(show: MediaHeadData | null | undefined) {
  return mediaHead(show, SHOW_FALLBACK_HEAD, "Browse");
}
