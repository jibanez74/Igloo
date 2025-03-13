import { createFileRoute } from "@tanstack/solid-router";

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
  component: RouteComponent,
});

function RouteComponent() {
  return <div>Hello "/_auth/movies/$movieID/play"!</div>;
}
