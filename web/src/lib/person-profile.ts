import { getSafeRedirect } from "@/lib/redirect-utils";
import type { PersonData, PersonTitle } from "@/types/people";

export const PERSON_TITLES_PER_PAGE = 24;

export function personReturnTo(value: unknown): string {
  if (typeof value !== "string" || getSafeRedirect(value) !== value) return "/";
  const path = value.split(/[?#]/, 1)[0];
  return /^\/(?:movies\/(?:in-theaters\/)?[1-9]\d*|tv-shows\/[1-9]\d*)\/?$/.test(
    path,
  )
    ? value
    : "/";
}

export function personDate(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const date = new Date(`${value}T00:00:00Z`);
  if (
    !Number.isFinite(date.getTime()) ||
    date.toISOString().slice(0, 10) !== value
  )
    {return null;}
  return date;
}

export function personAge(
  birthday: string,
  deathday: string,
  now = new Date(),
): number | null {
  const birth = personDate(birthday);
  const end = deathday ? personDate(deathday) : now;
  if (!birth || !end || end < birth) return null;
  let age = end.getUTCFullYear() - birth.getUTCFullYear();
  if (
    end.getUTCMonth() < birth.getUTCMonth() ||
    (end.getUTCMonth() === birth.getUTCMonth() &&
      end.getUTCDate() < birth.getUTCDate())
  )
    {age--;}
  return age;
}

export function personDateLabel(value: string): string | null {
  const date = personDate(value);
  return date
    ? new Intl.DateTimeFormat("en", {
        dateStyle: "long",
        timeZone: "UTC",
      }).format(date)
    : null;
}

export function knownForTitles(
  titles: PersonTitle[],
  department: string,
): PersonTitle[] {
  const matches = (title: PersonTitle) =>
    Boolean(
      department && title.roles.some((role) => role.department === department),
    );
  return [...titles]
    .sort(
      (a, b) =>
        Number(matches(b)) - Number(matches(a)) ||
        b.popularity - a.popularity ||
        a.title.localeCompare(b.title),
    )
    .slice(0, 8);
}

export function filterPersonTitles(
  titles: PersonTitle[],
  media: string,
  work: string,
  library: string,
): PersonTitle[] {
  return titles
    .filter(
      (title) =>
        (media === "all" || title.media_type === media) &&
        (work === "all" || title.roles.some((role) => role.kind === work)) &&
        (library === "all" ||
          (library === "available"
            ? title.library_id !== null
            : title.library_id === null)),
    )
    .sort(
      (a, b) =>
        b.date.localeCompare(a.date) ||
        a.title.localeCompare(b.title) ||
        a.media_type.localeCompare(b.media_type) ||
        a.tmdb_id - b.tmdb_id,
    );
}

export function personExternalLinks(
  person: PersonData["person"],
): { label: string; href: string }[] {
  const links = [
    {
      label: "TMDB",
      href: `https://www.themoviedb.org/person/${person.tmdb_id}`,
    },
  ];
  try {
    const website = new URL(person.homepage);
    if (website.protocol === "https:" || website.protocol === "http:")
      {links.unshift({ label: "Official website", href: website.href });}
  } catch {
    /* A missing or invalid homepage has no link. */
  }
  const ids = person.external_ids;
  const profiles = [
    ["IMDb", ids.imdb_id, "https://www.imdb.com/name/"],
    ["Facebook", ids.facebook_id, "https://www.facebook.com/"],
    ["Instagram", ids.instagram_id, "https://www.instagram.com/"],
    ["X / Twitter", ids.twitter_id, "https://x.com/"],
    ["TikTok", ids.tiktok_id, "https://www.tiktok.com/@"],
    ["YouTube", ids.youtube_id, "https://www.youtube.com/channel/"],
    ["Wikidata", ids.wikidata_id, "https://www.wikidata.org/wiki/"],
  ];
  for (const [label, id, base] of profiles) {
    if (id) links.push({ label, href: base + encodeURIComponent(id) });
  }
  return links;
}
