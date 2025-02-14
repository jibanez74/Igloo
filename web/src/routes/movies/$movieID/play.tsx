import { createFileRoute } from "@tanstack/react-router";
import ReactPlayer from "react-player";

type PlayParams = {
  title: string;
  thumb: string;
};

export const Route = createFileRoute("/movies/$movieID/play")({
  validateSearch: (search: Record<string, unknown>): PlayParams => ({
    title: String(search.title),
    thumb: String(search.thumb),
  }),
  loaderDeps: ({ search }) => search,
  component: PlayMoviePage,
});

function PlayMoviePage() {
  const { movieID } = Route.useParams();
  const { title, thumb } = Route.useSearch<PlayParams>();

  console.log(thumb);
  console.log(title);

  return (
    <div className='min-h-screen bg-slate-900 flex items-center justify-center'>
      <div className='w-full max-w-7xl aspect-video'>
        <ReactPlayer
          url={`/api/v1/movies/stream/${movieID}`}
          controls
          width='100%'
          height='100%'
          config={{
            file: {
              forceVideo: true,
            },
          }}
        />
      </div>
    </div>
  );
}
