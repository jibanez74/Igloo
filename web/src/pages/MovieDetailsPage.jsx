import { Suspense, suspense } from "react";
import { useNavigate, Await, defer, useLoaderData } from "react-router-dom";
import queryClient from "../utils/queryClient";
import { getMovieByID } from "../http/movieRequest";
import Spinner from "../shared/Spinner";

export default function MovieDetailsPage() {
  const { data } = useLoaderData();

  const playMovie = () => {
  };

  return (
    <Suspense fallback={<Spinner />}>
      <Await resolve={data}>
        {loadedData => (
          <>
            <h1>{loadedData.title}</h1>

            <p>{loadedData.filePath}</p>

            <p>{loadedData.container}</p>

            <button type='button' onClick={playMovie}>
              play movie
            </button>
          </>
        )}
      </Await>
    </Suspense>
  );
}

export async function loader({ params }) {
  return defer({
    data: await queryClient.fetchQuery({
      queryKey: ["movie", params.id],
      queryFn: () => getMovieByID(params.id),
    }),
  });
}
