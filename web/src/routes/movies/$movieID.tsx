import { createFileRoute, useLoaderData } from "@tanstack/react-router";
import type { Movie } from "@/types/Movie";
import type { Res } from "@/types/Response";

export const Route = createFileRoute("/movies/$movieID")({
  component: MovieDetailsPage,
  loader: async ({ params }): Promise<Movie> => {
    const res = await fetch(`/api/v1/movie/by-id/${params.movieID}`);

    const r: Res<Movie> = await res.json();

    if (r.error) {
      throw new Error(`${res.status} - ${r.message}`);
    }

    return r.data!;
  },
});

function MovieDetailsPage() {
  const movie = useLoaderData({
    from: "/movies/$movieID",
  });

  return (
    <div className='container'>
      <div className='back'>
        <Link
          className='inline-block px-4 py-2 border border-secondary rounded-md text-white cursor-pointer bg-transparent transition-all duration-300 ease-in-out'
          to='/movies/'
          search={prev => prev}
        >
          Back to Movies
        </Link>
      </div>

      <section title="movie details section">
        {/* movie details top */}
        <div className="flex justify-center items-center my-[50px] mb-[30px]">

        </div>
        {/* end of movie details top */}
      </section>
    </div>
  );
}
