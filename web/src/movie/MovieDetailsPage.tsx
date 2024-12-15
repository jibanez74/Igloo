import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import getError from "../lib/getError";
import api from "../lib/api";
import getImgSrc from "../lib/getImgSrc";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Spinner from "react-bootstrap/Spinner";
import Alert from "react-bootstrap/Alert";
import Badge from "react-bootstrap/Badge";
import Card from "react-bootstrap/Card";
import Modal from "react-bootstrap/Modal";
import Image from "react-bootstrap/Image";
import { FaArrowLeft, FaStar, FaPlay } from "react-icons/fa";
import formatDollars from "../lib/formatDollars";
import formatDate from "../lib/formatDate";
import type { Movie } from "../types/Movie";

type MovieResponse = {
  movie: Movie;
};

export default function MovieDetailsPage() {
  const { id } = useParams();

  const navigate = useNavigate();

  const [selectedVideo, setSelectedVideo] = useState<string | null>(null);
  const [playingVideoId, setPlayingVideoId] = useState<string | null>(null);

  const getYouTubeId = (url: string) => {
    const match = url.match(/[?&]v=([^&]+)/);
    return match ? match[1] : null;
  };

  const handleVideoSelect = (url: string) => {
    setSelectedVideo(url);
    setPlayingVideoId(getYouTubeId(url));
  };

  const handleCloseModal = () => {
    setSelectedVideo(null);
    setPlayingVideoId(null);
  };

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["movie", id],
    queryFn: async () => {
      try {
        const { data } = await api.get<MovieResponse>(`/movies/${id}`);

        if (!data.movie) {
          throw new Error("the server did not return a movie");
        }

        return data.movie;
      } catch (err) {
        throw getError(err);
      }
    },
  });

  if (isPending) {
    return (
      <Container className='py-5'>
        <Button
          variant='outline-primary'
          onClick={() => navigate(-1)}
          className='mb-4'
        >
          <FaArrowLeft className='me-2' /> Back
        </Button>
        <div
          className='d-flex justify-content-center align-items-center'
          style={{ minHeight: "400px" }}
        >
          <Spinner animation='border' role='status' variant='primary'>
            <span className='visually-hidden'>Loading movie details...</span>
          </Spinner>
        </div>
      </Container>
    );
  }

  if (isError) {
    return (
      <Container className='py-5'>
        <Button
          variant='outline-primary'
          onClick={() => navigate(-1)}
          className='mb-4'
        >
          <FaArrowLeft className='me-2' /> Back
        </Button>
        <Alert variant='danger'>
          {error instanceof Error ? error.message : "An error occurred"}
        </Alert>
      </Container>
    );
  }

  return (
    <Container className='py-5'>
      <Button
        variant='outline-primary'
        onClick={() => navigate(-1)}
        className='mb-4'
      >
        <FaArrowLeft className='me-2' /> Back
      </Button>

      {/* Movie Header */}
      <header className='mb-5'>
        <Row>
          <Col md={4} lg={3}>
            <Image
              src={getImgSrc(data.thumb)}
              alt={`${data.title} poster`}
              fluid
              rounded
              className='shadow-sm mb-4'
            />
          </Col>
          <Col md={8} lg={9}>
            <div className='d-flex flex-column h-100'>
              <h1 className='mb-2'>
                {data.title}
                {data.audienceRating > 0 && (
                  <Badge bg='warning' text='dark' className='ms-3 align-middle'>
                    <FaStar className='me-1' />
                    {data.audienceRating.toFixed(1)}
                  </Badge>
                )}
              </h1>
              {data.tagLine && (
                <p className='lead text-muted mb-3 fst-italic'>
                  {data.tagLine}
                </p>
              )}

              {/* Genres Section */}
              {data.genres.length > 0 && (
                <div className='mb-3'>
                  <small className='text-muted'>Genres: </small>
                  <span>
                    {data.genres.map((genre, index) => (
                      <span key={genre.ID}>
                        {genre.title}
                        {index < data.genres.length - 1 ? ", " : ""}
                      </span>
                    ))}
                  </span>
                </div>
              )}

              <p className='mb-4'>{data.summary}</p>
              <div className='mt-auto'>
                <Row className='g-3'>
                  <Col sm={6} md={4}>
                    <div className='d-flex flex-column'>
                      <small className='text-muted'>Release Date</small>
                      <span>{formatDate(data.releaseDate)}</span>
                    </div>
                  </Col>
                  <Col sm={6} md={4}>
                    <div className='d-flex flex-column'>
                      <small className='text-muted'>Runtime</small>
                      <span>
                        {Math.floor(data.runTime / 60)}h {data.runTime % 60}m
                      </span>
                    </div>
                  </Col>
                  <Col sm={6} md={4}>
                    <div className='d-flex flex-column'>
                      <small className='text-muted'>Rating</small>
                      <span>{data.contentRating}</span>
                    </div>
                  </Col>
                  {data.budget > 0 && (
                    <Col sm={6} md={4}>
                      <div className='d-flex flex-column'>
                        <small className='text-muted'>Budget</small>
                        <span>{formatDollars(data.budget)}</span>
                      </div>
                    </Col>
                  )}
                  {data.revenue > 0 && (
                    <Col sm={6} md={4}>
                      <div className='d-flex flex-column'>
                        <small className='text-muted'>Revenue</small>
                        <span>{formatDollars(data.revenue)}</span>
                      </div>
                    </Col>
                  )}
                </Row>
              </div>
            </div>
          </Col>
        </Row>
      </header>

      {/* Movie Details */}
      <Row className='g-4'>
        {/* Cast Section */}
        <Col xs={12}>
          <section>
            <h2 className='h4 mb-4'>Cast</h2>
            <Row xs={2} md={3} lg={4} className='g-4'>
              {data.castList.map(cast => (
                <Col key={cast.ID}>
                  <Card className='h-100 border-0 shadow-sm'>
                    <Card.Body className='d-flex flex-column'>
                      <div className='text-center mb-3'>
                        {cast.artist.thumb ? (
                          <Image
                            src={getImgSrc(cast.artist.thumb)}
                            alt={cast.artist.name}
                            roundedCircle
                            style={{
                              width: 80,
                              height: 80,
                              objectFit: "cover",
                            }}
                          />
                        ) : (
                          <div
                            className='rounded-circle bg-light d-flex align-items-center justify-content-center'
                            style={{ width: 80, height: 80, margin: "0 auto" }}
                          >
                            <i
                              className='bi bi-person text-muted'
                              style={{ fontSize: "2rem" }}
                            ></i>
                          </div>
                        )}
                      </div>
                      <h3 className='h6 text-center mb-1'>
                        {cast.artist.name}
                      </h3>
                      <p className='text-muted text-center small mb-0'>
                        {cast.character}
                      </p>
                    </Card.Body>
                  </Card>
                </Col>
              ))}
            </Row>
          </section>
        </Col>

        {/* Crew Section */}
        <Col xs={12}>
          <section>
            <h2 className='h4 mb-4'>Crew</h2>
            <Row xs={2} md={3} lg={4} className='g-4'>
              {data.crewList.map(crew => (
                <Col key={crew.ID}>
                  <Card className='h-100 border-0 shadow-sm'>
                    <Card.Body className='d-flex flex-column'>
                      <div className='text-center mb-3'>
                        {crew.artist.thumb ? (
                          <Image
                            src={getImgSrc(crew.artist.thumb)}
                            alt={crew.artist.name}
                            roundedCircle
                            style={{
                              width: 80,
                              height: 80,
                              objectFit: "cover",
                            }}
                          />
                        ) : (
                          <div
                            className='rounded-circle bg-light d-flex align-items-center justify-content-center'
                            style={{ width: 80, height: 80, margin: "0 auto" }}
                          >
                            <i
                              className='bi bi-person text-muted'
                              style={{ fontSize: "2rem" }}
                            ></i>
                          </div>
                        )}
                      </div>
                      <h3 className='h6 text-center mb-1'>
                        {crew.artist.name}
                      </h3>
                      <p className='text-muted text-center small mb-0'>
                        {crew.job}
                      </p>
                    </Card.Body>
                  </Card>
                </Col>
              ))}
            </Row>
          </section>
        </Col>

        {/* Studios */}
        {data.studios.length > 0 && (
          <Col xs={12}>
            <section>
              <h2 className='h4 mb-4'>Studios</h2>
              <Row xs={2} md={3} lg={4} className='g-4'>
                {data.studios.map(studio => (
                  <Col key={studio.ID}>
                    <Card className='h-100 border-0 shadow-sm'>
                      <Card.Body className='d-flex flex-column'>
                        <div className='text-center mb-3'>
                          {studio.logo ? (
                            <Image
                              src={getImgSrc(studio.logo)}
                              alt={studio.name}
                              style={{
                                width: "auto",
                                height: 80,
                                objectFit: "contain",
                              }}
                            />
                          ) : (
                            <div
                              className='bg-light d-flex align-items-center justify-content-center'
                              style={{ height: 80 }}
                            >
                              <i
                                className='bi bi-building text-muted'
                                style={{ fontSize: "2rem" }}
                              ></i>
                            </div>
                          )}
                        </div>
                        <h3 className='h6 text-center mb-0'>{studio.name}</h3>
                      </Card.Body>
                    </Card>
                  </Col>
                ))}
              </Row>
            </section>
          </Col>
        )}

        {/* Videos and Extras */}
        {data.extras.length > 0 && (
          <Col xs={12}>
            <section>
              <h2 className='h4 mb-4'>Videos & Extras</h2>
              <Row xs={1} md={2} lg={3} className='g-4'>
                {data.extras.map(extra => (
                  <Col key={extra.ID}>
                    <Card className='h-100 border-0 shadow-sm'>
                      <Card.Body>
                        <h3 className='h6 mb-3'>{extra.title}</h3>
                        <div className='d-grid'>
                          <Button
                            variant='primary'
                            onClick={() => handleVideoSelect(extra.url)}
                          >
                            <FaPlay className='me-2' /> Watch {extra.kind}
                          </Button>
                        </div>
                      </Card.Body>
                    </Card>
                  </Col>
                ))}
              </Row>
            </section>
          </Col>
        )}
      </Row>

      {/* Video Modal */}
      <Modal
        show={!!selectedVideo}
        onHide={handleCloseModal}
        size='lg'
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>
            {data?.extras.find(e => e.url === selectedVideo)?.title}
          </Modal.Title>
        </Modal.Header>
        <Modal.Body className='p-0'>
          <div className='ratio ratio-16x9'>
            {playingVideoId && (
              <iframe
                src={`https://www.youtube.com/embed/${playingVideoId}?autoplay=1`}
                title='YouTube video player'
                allow='accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture'
                allowFullScreen
              ></iframe>
            )}
          </div>
        </Modal.Body>
      </Modal>
    </Container>
  );
}
