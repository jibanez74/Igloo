import type { LibraryMovieCrewType } from "@/types/movies";

const KEY_CREW_WRITERS_CAP = 3;

/** The people a movie bills above its full crew: the director and the first
 *  few writers. Either may be missing. */
export function movieKeyCrew(crew: LibraryMovieCrewType[]) {
  return {
    director: crew.find(c => c.job === "Director"),
    writers: crew
      .filter(c => c.department === "Writing")
      .slice(0, KEY_CREW_WRITERS_CAP),
  };
}

export function movieHasKeyCrew(crew: LibraryMovieCrewType[]) {
  const { director, writers } = movieKeyCrew(crew);
  return Boolean(director) || writers.length > 0;
}

/**
 * What a detail page calls its crew section. "Key Crew" promises a billed
 * summary (a movie's director and writers, a show's creators); a section that
 * holds only the full-crew disclosure is plain "Crew". The heading and the
 * skip link that targets it read the same word, so both come from here.
 */
export function crewSectionLabels(hasKeyCrew: boolean) {
  return hasKeyCrew
    ? { heading: "Key Crew", skipLabel: "Skip to key crew" }
    : { heading: "Crew", skipLabel: "Skip to crew" };
}
