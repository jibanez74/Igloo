import { useQuery } from "@tanstack/react-query";
import {
  createFileRoute,
  Link,
  stripSearchParams,
} from "@tanstack/react-router";
import { ArrowLeft, Film, User } from "lucide-react";
import { useState } from "react";

import DetailSkipLinks from "@/components/shared/DetailSkipLinks";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import MediaDetailGuard from "@/components/shared/MediaDetailGuard";
import PosterCard from "@/components/shared/PosterCard";
import RetryButton from "@/components/shared/RetryButton";
import ScrollRail from "@/components/shared/ScrollRail";
import SkeletonStatus from "@/components/shared/SkeletonStatus";
import { Button, buttonVariants } from "@/components/ui/button";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
  DETAIL_SECTION_HEADING_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
  PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS,
  TMDB_POSTER_SIZE,
  TMDB_PROFILE_SIZE,
} from "@/lib/constants";
import {
  filterPersonTitles,
  knownForTitles,
  PERSON_TITLES_PER_PAGE,
  personAge,
  personDateLabel,
  personExternalLinks,
} from "@/lib/person-profile";
import { personQueryOpts } from "@/lib/query-opts";
import { routeHead } from "@/lib/route-head";
import { parseRouteId } from "@/lib/route-id";
import { personSearchSchema } from "@/lib/route-search";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { PersonData, PersonTitle } from "@/types/people";

export const Route = createFileRoute("/_auth/people/$id")({
  validateSearch: personSearchSchema,
  search: {
    middlewares: [
      stripSearchParams({
        media: "all",
        work: "all",
        library: "all",
        page: 1,
        returnTo: "/",
      }),
    ],
  },
  loader: async ({ context, params }) => {
    const id = parseRouteId(params.id);
    if (id == null) return { person: null };
    const response = await context.queryClient.ensureQueryData(
      personQueryOpts(id),
    );
    return { person: response.error ? null : response.data.person };
  },
  head: ({ loaderData }) =>
    routeHead(
      loaderData?.person?.name ?? "Person",
      loaderData?.person?.biography,
    ),
  pendingComponent: PersonSkeleton,
  component: PersonPage,
});

function PersonSkeleton() {
  return (
    <SkeletonStatus
      label="Loading person profile"
      className={MOTION_LOADING_STATE_CLASS}
    >
      <div className="mb-6 h-9 w-24 rounded-md bg-muted" />
      <div className="flex flex-col gap-6 md:flex-row md:gap-8">
        <div className="aspect-2/3 w-48 shrink-0 rounded-xl bg-muted md:w-64" />
        <div className="min-w-0 flex-1 space-y-6">
          <div className="h-12 w-3/4 rounded-md bg-muted" />
          <div className="h-5 w-32 rounded-md bg-muted" />
          <div className="h-40 rounded-xl bg-muted" />
          <div className="h-24 rounded-xl bg-muted" />
        </div>
      </div>
      <div className="mt-8 h-7 w-40 rounded-md bg-muted" />
      <div className="mt-4 space-y-3">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="h-24 rounded-xl bg-muted" />
        ))}
      </div>
    </SkeletonStatus>
  );
}

function PersonPage() {
  const { id } = Route.useParams();
  const personId = parseRouteId(id);
  const query = useQuery(personQueryOpts(personId ?? 0));
  return (
    <MediaDetailGuard
      id={personId}
      noun="person"
      back="home"
      data={query.data}
      payload={query.data?.data ?? null}
      isPending={query.isPending}
      isError={query.isError}
      skeleton={<PersonSkeleton />}
      onRetry={() => void query.refetch()}
    >
      {(data) => (
        <PersonContent
          key={id}
          data={data}
          onRetry={() => void query.refetch()}
          isFetching={query.isFetching}
        />
      )}
    </MediaDetailGuard>
  );
}

function PersonImage({
  path,
  name,
  thumbnail = false,
}: {
  path: string;
  name: string;
  thumbnail?: boolean;
}) {
  const url = buildTmdbImageUrl(
    path,
    thumbnail ? TMDB_PROFILE_SIZE : "original",
  );
  const { showPoster, onError } = usePosterFallback(url);
  return (
    <div className="aspect-2/3 overflow-hidden rounded-xl border border-primary/20 bg-muted">
      {showPoster ? (
        <img
          src={url}
          alt={`Photo of ${name}`}
          width={300}
          height={450}
          decoding="async"
          loading={thumbnail ? "lazy" : undefined}
          fetchPriority={thumbnail ? "low" : undefined}
          className="size-full object-cover"
          onError={onError}
        />
      ) : (
        <div
          role="img"
          aria-label={`No photo available for ${name}`}
          className="flex size-full items-center justify-center"
        >
          <User className="size-12 text-muted-foreground" aria-hidden="true" />
        </div>
      )}
    </div>
  );
}

function Biography({ text }: { text: string }) {
  const [expanded, setExpanded] = useState(false);
  const long = text.length > 600;
  const shown =
    long && !expanded ? `${text.slice(0, 600).replace(/\s+\S*$/, "")}…` : text;
  return (
    <section aria-labelledby="biography-heading" className="mt-6">
      <h2
        id="biography-heading"
        tabIndex={-1}
        className={DETAIL_SECTION_HEADING_CLASS}
      >
        Biography
      </h2>
      <div
        id="person-biography"
        className="space-y-4 text-sm/relaxed text-muted-foreground sm:text-base"
      >
        {shown ? (
          shown.split(/\n\s*\n/).map((paragraph, i) => (
            <p key={i} className="whitespace-pre-line">
              {paragraph}
            </p>
          ))
        ) : (
          <p>No biography available.</p>
        )}
      </div>
      {long && (
        <Button
          variant="outline"
          size="sm"
          className="mt-3"
          aria-expanded={expanded}
          aria-controls="person-biography"
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Show less" : "Read more"}
        </Button>
      )}
    </section>
  );
}

function titleLink(title: PersonTitle) {
  if (title.library_id !== null) {
    return {
      to:
        title.media_type === "movie"
          ? ("/movies/$id" as const)
          : ("/tv-shows/$id" as const),
      params: { id: String(title.library_id) },
    };
  }
  return {
    href: `https://www.themoviedb.org/${title.media_type}/${title.tmdb_id}`,
  };
}

function FilmographyRow({ title }: { title: PersonTitle }) {
  const url = buildTmdbImageUrl(title.poster_path, TMDB_POSTER_SIZE);
  const { showPoster, onError } = usePosterFallback(url);
  const link = titleLink(title);
  const content = (
    <>
      <div className="aspect-2/3 w-12 shrink-0 overflow-hidden rounded-md bg-muted sm:w-16">
        {showPoster ? (
          <img
            src={url}
            alt=""
            width={100}
            height={150}
            loading="lazy"
            decoding="async"
            fetchPriority="low"
            className="size-full object-cover"
            onError={onError}
          />
        ) : (
          <div className="flex size-full items-center justify-center">
            <Film aria-hidden="true" className="size-6 text-muted-foreground" />
          </div>
        )}
      </div>
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold wrap-anywhere">{title.title}</h3>
        <p className="text-sm text-muted-foreground">
          {title.media_type === "movie" ? "Movie" : "TV show"} ·{" "}
          {title.date.slice(0, 4) || "Undated"}
        </p>
        <ul
          className="mt-1 flex list-none flex-wrap gap-x-3 gap-y-1 text-sm text-muted-foreground"
          role="list"
          aria-label="Roles"
        >
          {title.roles.map((role, i) => (
            <li key={i}>
              {role.kind === "acting"
                ? `Acting${role.role ? `: ${role.role}` : ""}`
                : role.role || role.department || "Crew"}
            </li>
          ))}
        </ul>
        {title.library_id !== null && (
          <p className="mt-1 text-xs font-medium text-primary">
            In your library
          </p>
        )}
      </div>
    </>
  );
  const className = cn(
    "flex items-center gap-3 rounded-xl border border-border bg-card p-3 hover:border-primary/40",
    FOCUS_VISIBLE_RING_CLASS,
  );
  return (
    <li>
      {"href" in link ? (
        <a
          href={link.href}
          target="_blank"
          rel="noopener noreferrer"
          className={className}
          aria-label={`${title.title} on TMDB (opens in a new tab)`}
        >
          {content}
        </a>
      ) : (
        <Link {...link} className={className}>
          {content}
        </Link>
      )}
    </li>
  );
}

function PersonContent({
  data,
  onRetry,
  isFetching,
}: {
  data: PersonData;
  onRetry: () => void;
  isFetching: boolean;
}) {
  const { person, filmography, photos, enrichment_status: status } = data;
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const knownFor = knownForTitles(filmography, person.known_for_department);
  const filtered = filterPersonTitles(
    filmography,
    search.media,
    search.work,
    search.library,
  );
  const totalPages = Math.max(
    1,
    Math.ceil(filtered.length / PERSON_TITLES_PER_PAGE),
  );
  const page = Math.min(search.page, totalPages);
  const visible = filtered.slice(
    (page - 1) * PERSON_TITLES_PER_PAGE,
    page * PERSON_TITLES_PER_PAGE,
  );
  const age = personAge(person.birthday, person.deathday);
  const birthday = personDateLabel(person.birthday);
  const deathday = personDateLabel(person.deathday);
  const gender = ["", "Female", "Male", "Non-binary"][person.gender];
  const facts: [string, string | null][] = [
    ["Birthday", birthday],
    [
      person.deathday ? "Age at death" : "Age",
      age === null ? null : String(age),
    ],
    ["Place of birth", person.place_of_birth],
    ["Death date", deathday],
    ["Gender", gender],
    ["Also known as", person.also_known_as.join(", ")],
  ];
  const links = personExternalLinks(person);
  const changeFilter = (key: "media" | "work" | "library", value: string) =>
    void navigate({
      search: (previous) => ({ ...previous, [key]: value, page: 1 }),
    });
  const partialMessage =
    status === "unavailable"
      ? "TMDB is temporarily unavailable. Showing stored information and library credits."
      : status === "unconfigured"
        ? "TMDB is not configured. Showing stored information and library credits."
        : "This person is no longer available on TMDB. Showing stored information and library credits.";
  return (
    <div className={cn("min-w-0", DETAIL_PAGE_CONTENT_ENTER_CLASS)}>
      <DetailSkipLinks
        titleHref="#person-heading"
        titleLabel="Skip to person"
        sections={[
          { href: "#biography-heading", label: "Skip to biography" },
          knownFor.length > 0 && {
            href: "#known-for-heading",
            label: "Skip to known for",
          },
          { href: "#filmography-heading", label: "Skip to filmography" },
          photos.length > 0 && {
            href: "#photos-heading",
            label: "Skip to photos",
          },
        ]}
      />
      <nav aria-label="Page navigation" className="mb-6">
        <Link
          to={search.returnTo}
          className={buttonVariants({ variant: "outline", size: "sm" })}
        >
          <ArrowLeft aria-hidden="true" className="size-4" />
          {search.returnTo === "/" ? "Back to Home" : "Back"}
        </Link>
      </nav>
      {status !== "available" && (
        <div className="mb-6 rounded-xl border border-primary/20 bg-primary/5 p-4">
          <p role="status" className="text-sm text-muted-foreground">
            {isFetching ? "Refreshing person information…" : partialMessage}
          </p>
          {status === "unavailable" && !isFetching && (
            <RetryButton onRetry={onRetry} />
          )}
        </div>
      )}
      <div className="flex flex-col gap-6 md:flex-row md:gap-8">
        <div className="w-48 shrink-0 md:w-64">
          <PersonImage path={person.profile_path} name={person.name} />
        </div>
        <div className="min-w-0 flex-1">
          <h1
            id="person-heading"
            tabIndex={-1}
            className={cn(
              "rounded-sm text-3xl font-semibold tracking-tight wrap-anywhere text-foreground sm:text-5xl",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            {person.name}
          </h1>
          {person.known_for_department && (
            <p className="mt-3 text-lg text-primary">
              {person.known_for_department}
            </p>
          )}
          <Biography text={person.biography} />
          {facts.some(([, value]) => value) && (
            <section aria-labelledby="personal-info-heading" className="mt-6">
              <h2
                id="personal-info-heading"
                className={DETAIL_SECTION_HEADING_CLASS}
              >
                Personal information
              </h2>
              <dl className="grid gap-4 sm:grid-cols-2">
                {facts.map(
                  ([label, value]) =>
                    value && (
                      <div key={label} className="min-w-0">
                        <dt className="text-sm text-muted-foreground">
                          {label}
                        </dt>
                        <dd className="mt-1 text-sm wrap-anywhere">{value}</dd>
                      </div>
                    ),
                )}
              </dl>
            </section>
          )}
          <nav aria-label="External profiles" className="mt-6">
            <ul className="flex list-none flex-wrap gap-3" role="list">
              {links.map((link) => (
                <li key={link.label}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={cn(
                      "rounded-sm text-sm text-primary underline",
                      FOCUS_VISIBLE_RING_CLASS,
                    )}
                    aria-label={`${person.name} on ${link.label} (opens in a new tab)`}
                  >
                    {link.label}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </div>
      {knownFor.length > 0 && (
        <section className="mt-8" aria-labelledby="known-for-heading">
          <h2
            id="known-for-heading"
            tabIndex={-1}
            className={DETAIL_SECTION_HEADING_CLASS}
          >
            Known for
          </h2>
          <ScrollRail label="known for" asChild>
            <ul className="list-none gap-4 pb-4" role="list">
              {knownFor.map((title) => (
                <li
                  key={`${title.media_type}:${title.tmdb_id}:${title.library_id}`}
                  className="w-36 shrink-0 sm:w-40"
                >
                  <PosterCard
                    detailsLink={titleLink(title)}
                    title={title.title}
                    posterUrl={buildTmdbImageUrl(
                      title.poster_path,
                      TMDB_POSTER_SIZE,
                    )}
                    fallbackIcon={Film}
                    subtitle={`${title.date.slice(0, 4) || "Undated"}${title.library_id !== null ? " · In your library" : ""}`}
                    detailsLabel={
                      title.library_id !== null
                        ? `View ${title.title} in your library`
                        : `${title.title} on TMDB (opens in a new tab)`
                    }
                  />
                </li>
              ))}
            </ul>
          </ScrollRail>
        </section>
      )}
      <section className="mt-8" aria-labelledby="filmography-heading">
        <h2
          id="filmography-heading"
          tabIndex={-1}
          className={DETAIL_SECTION_HEADING_CLASS}
        >
          Filmography
        </h2>
        <div className="mb-4 grid gap-4 sm:grid-cols-3">
          {(
            [
              [
                "media",
                "Media type",
                [
                  ["all", "All titles"],
                  ["movie", "Movies"],
                  ["tv", "TV shows"],
                ],
              ],
              [
                "work",
                "Work",
                [
                  ["all", "All work"],
                  ["acting", "Acting"],
                  ["crew", "Crew"],
                ],
              ],
              [
                "library",
                "Availability",
                [
                  ["all", "All credits"],
                  ["available", "In your library"],
                  ["unavailable", "Outside your library"],
                ],
              ],
            ] as const
          ).map(([key, label, options]) => (
            <div key={key}>
              <label
                htmlFor={`person-${key}`}
                className="mb-2 block text-sm font-medium"
              >
                {label}
              </label>
              <select
                id={`person-${key}`}
                value={search[key]}
                onChange={(event) => changeFilter(key, event.target.value)}
                className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              >
                {options.map(([value, text]) => (
                  <option key={value} value={value}>
                    {text}
                  </option>
                ))}
              </select>
            </div>
          ))}
        </div>
        <p className="mb-4 text-sm text-muted-foreground">
          {filtered.length} titles · Newest first
          {filtered.length > 0 && ` · Page ${page} of ${totalPages}`}
        </p>
        <LiveAnnouncer
          message={
            filtered.length
              ? `${filtered.length} titles, page ${page} of ${totalPages}`
              : "No credits match these filters."
          }
          announcementKey={`${search.media}:${search.work}:${search.library}:${page}`}
        />
        {visible.length ? (
          <ul className="list-none space-y-3" role="list">
            {visible.map((title) => (
              <FilmographyRow
                key={`${title.media_type}:${title.tmdb_id}:${title.library_id}`}
                title={title}
              />
            ))}
          </ul>
        ) : (
          <p className="py-8 text-center text-muted-foreground">
            No credits match these filters.
          </p>
        )}
        {totalPages > 1 && (
          <div className="mt-6">
            <LibraryPagination
              currentPage={page}
              totalPages={totalPages}
              onPageChange={(next) =>
                void navigate({
                  search: (previous) => ({ ...previous, page: next }),
                })
              }
            />
          </div>
        )}
      </section>
      {photos.length > 0 && (
        <section className="mt-8" aria-labelledby="photos-heading">
          <h2
            id="photos-heading"
            tabIndex={-1}
            className={DETAIL_SECTION_HEADING_CLASS}
          >
            Photos
          </h2>
          <ScrollRail label="photos" asChild>
            <ul className="list-none gap-4 pb-4" role="list">
              {photos.map((path, i) => (
                <li key={path} className="w-32 shrink-0">
                  <a
                    href={buildTmdbImageUrl(path, "original")}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={cn("block rounded-xl", FOCUS_VISIBLE_RING_CLASS)}
                    aria-label={`View full-size photo ${i + 1} of ${person.name} (opens in a new tab)`}
                  >
                    <PersonImage path={path} name={person.name} thumbnail />
                  </a>
                </li>
              ))}
            </ul>
          </ScrollRail>
        </section>
      )}
    </div>
  );
}
