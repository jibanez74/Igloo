import { useQuery } from "@tanstack/react-query";
import { Music } from "lucide-react";
import { latestAlbumsQueryOpts } from "@/lib/query-opts";
import AlbumCard from "@/components/AlbumCard";
import HomeMediaSection from "@/components/HomeMediaSection";
import { HOME_ALBUM_GRID_CLASS } from "@/lib/constants";

export default function LatestAlbums() {
  const { data, isPending } = useQuery(latestAlbumsQueryOpts());

  const albums = data && !data.error ? (data.data?.albums ?? []) : [];
  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load albums. Please try again later."
    : undefined;

  return (
    <HomeMediaSection
      title="Recently Added Albums"
      headingId="recent-albums"
      items={albums}
      isPending={isPending}
      errorMessage={errorMessage}
      loadingLabel="Loading albums..."
      emptyTitle="No Albums Yet"
      emptyDescription="Your music library is empty. Add some albums to get started with your personal music collection."
      emptyIcon={Music}
      countNoun="album"
      gridClassName={HOME_ALBUM_GRID_CLASS}
      getKey={album => String(album.id)}
      renderItem={album => <AlbumCard album={album} />}
    />
  );
}
