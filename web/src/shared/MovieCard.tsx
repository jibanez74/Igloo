import { Link } from "react-router-dom";
import getImgSrc from "../lib/getImgSrc";
import Card from "react-bootstrap/Card";
import type { SimpleMovie } from "../types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
};

export default function MovieCard({ movie }: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);

  return (
    <Card className='w-100 h-100 bg-primary text-light rounded-1'>
      <Link to={`/movies/${movie.ID}`}>
        <Card.Img variant='top' src={imgSrc} className='rounded-top-1' />
      </Link>

      <Card.Body className='d-flex flex-column text-center'>
        <Card.Title
          className='text-truncate'
          style={{
            minHeight: "3rem",
            WebkitLineClamp: 2,
            display: "-webkit-box",
            WebkitBoxOrient: "vertical",
          }}
        >
          {movie.title}
        </Card.Title>

        <Card.Text>{movie.year}</Card.Text>
      </Card.Body>
    </Card>
  );
}
