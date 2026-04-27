import { useQuery } from "@tanstack/react-query";
import { Music } from "lucide-react";
import { latestAlbumsQueryOpts } from "@/lib/query-opts";
import AlbumCard from "@/components/AlbumCard";
import HomeMediaSection from "@/components/HomeMediaSection";

export default function LatestAlbums() {
  const { data, isPending } = useQuery(latestAlbumsQueryOpts());

  const albums = data && !data.error ? data.data.albums : [];
  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load albums. Please try again later."
    : undefined;

  const getAnnouncementMessage = () => {
    if (isPending) return undefined;
    if (hasError) return data.message || "Failed to load albums";
    if (albums.length === 0) return "No albums in your library";
    return undefined;
  };

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
      countLabel="albums"
      gridClassName="grid grid-cols-[repeat(auto-fit,minmax(min(8rem,100%),1fr))] gap-3 sm:gap-4 lg:grid-cols-[repeat(auto-fit,minmax(9rem,1fr))]"
      announcementMessage={getAnnouncementMessage()}
      getKey={album => String(album.id)}
      renderItem={album => <AlbumCard album={album} />}
    />
  );
}
