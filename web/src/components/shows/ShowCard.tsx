import { useQueryClient } from "@tanstack/react-query";
import { Tv } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { showDetailsQueryOpts } from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { LatestShowType } from "@/types";

type ShowCardProps = {
  show: LatestShowType;
};

// A show has no single thing to play (episodes do), so this card passes no
// play link - like InTheatersCard; the details page's hero picks the episode.
// It does prefetch its detail query on hover and focus, as every media card
// with a detail query does.
export default function ShowCard({ show }: ShowCardProps) {
  const { id, name, poster_path, premiere_year } = show;
  const queryClient = useQueryClient();

  const handlePrefetch = () => queryClient.prefetchQuery(showDetailsQueryOpts(id));

  const posterUrl =
    poster_path.Valid && poster_path.String !== ""
      ? buildTmdbImageUrl(poster_path.String, TMDB_POSTER_SIZE)
      : "";

  const cardAriaLabel = premiere_year.Valid
    ? `${name} ${premiere_year.Int64}`
    : name;

  return (
    <PosterCard
      detailsLink={{ to: "/tv-shows/$id", params: { id: String(id) } }}
      posterUrl={posterUrl}
      fallbackIcon={Tv}
      title={name}
      subtitle={premiere_year.Valid ? String(premiere_year.Int64) : undefined}
      detailsLabel={cardAriaLabel}
      onPrefetch={handlePrefetch}
    />
  );
}
