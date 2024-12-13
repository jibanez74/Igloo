import { useQuery } from "@tanstack/react-query";
import api from "../lib/api";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Spinner from "react-bootstrap/Spinner";
import Message from "../shared/Message";
import MovieCard from "../shared/MovieCard";
import type { SimpleMovie } from "../types/Movie";

type MoviesResponse = {
  movies: SimpleMovie[];
};

export default function LatestMovies() {
  const { data, isPending, isError } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/movies/latest");

        if (!data.movies) {
          throw new Error("no movies were fetched");
        }

        return data.movies;
      } catch (err) {
        console.error(err);
        throw err; // Re-throw to trigger error state
      }
    },
  });

  return (
    <section aria-labelledby='latest-movies-title'>
      {/* Header with fixed height to prevent layout shift */}
      <header style={{ minHeight: "48px" }} className='mb-4'>
        <h2 id='latest-movies-title' className='text-white m-0'>
          Latest Movies
        </h2>
      </header>

      {/* Content container with minimum height to prevent layout shift */}
      <div style={{ minHeight: "400px" }}>
        {isPending && (
          <div
            className='d-flex justify-content-center align-items-center py-5'
            role='status'
            aria-label='Loading latest movies'
          >
            <Spinner
              animation='border'
              variant='primary'
              style={{ width: "3rem", height: "3rem" }}
            >
              <span className='visually-hidden'>Loading latest movies...</span>
            </Spinner>
          </div>
        )}

        {isError && (
          <div role='alert' aria-live='polite'>
            <Message
              title='Loading Error'
              msg='Unable to fetch latest movies. Please try again later.'
              variant='danger'
            />
          </div>
        )}

        {!isPending && !isError && data && (
          <>
            {data.length === 0 ? (
              <div role='alert' aria-live='polite'>
                <Message
                  title='No Movies'
                  msg='There are no movies available at the moment.'
                  variant='info'
                />
              </div>
            ) : (
              <Row className='g-4' role='list' aria-label='Latest movies grid'>
                {data.map(movie => (
                  <Col
                    key={movie.ID}
                    sm={4}
                    md={3}
                    lg={2}
                    className='d-flex h-100'
                    role='listitem'
                  >
                    <MovieCard movie={movie} />
                  </Col>
                ))}
              </Row>
            )}
          </>
        )}
      </div>
    </section>
  );
}
