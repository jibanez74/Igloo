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
    <>
      <Row className='mb-4'>
        <Col>
          <h2 className='text-white'>Latest Movies</h2>
        </Col>
      </Row>

      {isPending && (
        <Row className='justify-content-center my-5'>
          <Col xs='auto'>
            <Spinner animation='border' role='status' variant='primary'>
              <span className='visually-hidden'>Loading...</span>
            </Spinner>
          </Col>
        </Row>
      )}

      {isError && (
        <Row className='justify-content-center my-4'>
          <Col xs={12} md={6}>
            <Message
              title='Loading Error'
              msg='Unable to fetch latest movies. Please try again later.'
              variant='danger'
            />
          </Col>
        </Row>
      )}

      {!isPending && !isError && data && (
        <>
          {data.length === 0 ? (
            <Row className='justify-content-center my-4'>
              <Col xs={12} md={6}>
                <Message
                  title='No Movies'
                  msg='There are no movies available at the moment.'
                  variant='info'
                />
              </Col>
            </Row>
          ) : (
            <Row className='g-4'>
              {data.map(m => (
                <Col key={m.ID} sm={4} md={3} lg={2} className='d-flex h-100'>
                  <MovieCard movie={m} />
                </Col>
              ))}
            </Row>
          )}
        </>
      )}
    </>
  );
}
