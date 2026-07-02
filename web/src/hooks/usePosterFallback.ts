import { useState } from "react";

export function usePosterFallback(posterUrl: string) {
  const [failedPosterUrl, setFailedPosterUrl] = useState<string | null>(null);
  const showPoster = posterUrl !== "" && failedPosterUrl !== posterUrl;
  const onError = () => setFailedPosterUrl(posterUrl);

  return { showPoster, onError };
}
