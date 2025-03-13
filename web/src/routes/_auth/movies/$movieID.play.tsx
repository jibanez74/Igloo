import { createFileRoute } from "@tanstack/solid-router";
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
  const { m3u8Url, pid, title, thumb } = search();

  console.log(`your pid is ${pid}`);

  return (
    <section>
      <HlsPlayer src={m3u8Url} />
    </section>
  );
}
