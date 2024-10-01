import { Suspense } from "react";
import { Await, useLoaderData, defer, useParams } from "react-router-dom";
import queryClient from "../utils/queryClient";
import { getMoviesWithPagination } from "../http/movieRequest";
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
                pageSize={data.pageSize}
                totalItems={data.totalCount}
                totalPages={data.totalPages}
                urlPrefix='/movies'
              />
            </>
          )}
        </Await>
      </Suspense>
    </section>
  );
}

