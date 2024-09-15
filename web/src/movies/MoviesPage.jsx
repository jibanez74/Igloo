import { Suspense } from "react";
import { Await, useLoaderData, defer, useParams } from "react-router-dom";
import queryClient from "../utils/queryClient";
import { getMoviesWithPagination } from "./httpMovie";
import Spinner from "../shared/Spinner";
import MovieCard from "../shared/MovieCard";
import Pagination from "../shared/Pagination";

export default function MoviesPage() {
  const { page } = useParams();
  const { data } = useLoaderData();

  return (
    <section className='container mx-auto'>
      <Suspense fallback={<Spinner />}>
        <Await resolve={data}>
          {data => (
            <>
              <div className='grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4'>
                {data.movies.map(m => (
                  <MovieCard key={m._id} movie={m} />
                ))}
              </div>

              <Pagination
                currentPage={Number(page) || 1}
                pageSize={24}
                totalItems={data.count}
                totalPages={data.pages}
                urlPrefix='/movies'
              />
            </>
          )}
        </Await>
      </Suspense>
    </section>
  );
}

async function getMovies(page = "1", keyword = "") {
  return queryClient.fetchQuery({
    queryKey: ["movies", page, keyword],
    queryFn: () => getMoviesWithPagination(page, keyword),
  });
}

export async function loader({ params }) {
  let { page, keyword } = params;

  if (!page) page = "1";
  if (keyword) keyword = "";

  return defer({
    data: await getMovies(page, keyword),
  });
}
