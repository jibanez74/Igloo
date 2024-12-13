import { useQuery } from "@tanstack/react-query";
import api from "../lib/api";
import getError from "../lib/getError";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Spinner from "react-bootstrap/Spinner";
import Alert from "react-bootstrap/Alert";
import MovieCard from "../shared/MovieCard";
import type { SimpleMovie } from "../types/Movie";

type MoviesResponse = {
  movies: SimpleMovie[];
  count: number;
};

export default function MoviesPage() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/movies/all");

        if (!data.movies) {
          throw new Error("the server did not return any movies");
        }

        return data;
      } catch (err) {
        throw getError(err);
      }
    },
  });

  return (
    <Container className='py-5'>
      <header style={{ minHeight: "60px" }} className='mb-4'>
        {!isPending && <h1>Movies ({data?.count || 0})</h1>}
      </header>

      {isPending && (
        <section
          aria-label='Loading'
          className='d-flex justify-content-center align-items-center'
          style={{ minHeight: "400px" }}
        >
          <div className='text-center' role='status'>
            <Spinner
              animation='border'
              variant='primary'
              style={{ width: "3rem", height: "3rem" }}
            >
              <span className='visually-hidden'>Loading movies...</span>
            </Spinner>
          </div>
        </section>
      )}

      {isError && (
        <section aria-label='Error Message'>
          <Alert variant='danger' className='my-4'>
            {error instanceof Error ? error.message : "An error occurred"}
          </Alert>
        </section>
      )}

      {!isPending && !isError && (
        <section aria-label='Movies Grid'>
          <Row sm={2} md={4} lg={6} className='g-4' role='list'>
            {data?.movies.map(movie => (
              <Col key={movie.ID} role='listitem'>
                <MovieCard movie={movie} />
              </Col>
            ))}
          </Row>

          {data?.movies.length === 0 && (
            <Alert variant='info' className='text-center mt-4'>
              No movies available
            </Alert>
          )}
        </section>
      )}
    </Container>
  );
}
