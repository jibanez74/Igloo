import { createFileRoute } from "@tanstack/solid-router";
import { onCleanup } from "solid-js";
import HlsPlayer from "../../../components/HlsPlayer";

type SearchParams = {
  title: string;
  thumb: string;
  pid: string;
  m3u8Url: string;
};

export const Route = createFileRoute("/_auth/movies/$movieID/play")({
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    title: String(search.title),
    thumb: String(search.thumb),
    pid: String(search.pid),
    m3u8Url: String(search.m3u8Url),
  }),
  loaderDeps: ({ search }) => search,
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const search = Route.useSearch();

  onCleanup(async () => {
    try {
      const res = await fetch(`/api/v1/ffmpeg/cancel-job/${search().pid}`, {
        method: "post",
        credentials: "include",
      });

      if (!res.ok) {
        console.error("Failed to cancel FFmpeg job:", await res.json());
      }
    } catch (err) {
      console.error("Error canceling FFmpeg job:", err);
    }
  });


  return (
    <section>
      <HlsPlayer
        src={search().m3u8Url}
        poster={search().thumb}
        title={search().title}
      />
    </section>
  );
}
