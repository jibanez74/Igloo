// Rating-tier chip colors, shared by MovieDetailsMetadataChips and
// InTheatersCard (previously duplicated with a /90 vs /80 mid-tier drift —
// standardized on /80 for clearer separation from the strong tier).

/** Critic score tiers on the warm aurora accent (design-system §3.2). */
export function criticRatingClass(score: number): string {
  if (score >= 7) return "bg-aurora text-aurora-foreground";
  if (score >= 5) return "bg-aurora/80 text-aurora-foreground";
  return "bg-muted text-foreground";
}

/** Audience score tiers on the glacier primary. Note bg-accent === bg-muted
 * in both themes today; the distinct classes preserve intent if accent ever
 * diverges. */
export function audienceRatingClass(score: number): string {
  if (score >= 7) return "bg-primary text-primary-foreground";
  if (score >= 5) return "bg-accent text-accent-foreground";
  return "bg-muted text-foreground";
}
