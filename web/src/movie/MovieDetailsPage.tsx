import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import getError from "../lib/getError";
import formatDollars from "../lib/formatDollars";
import formatDate from "../lib/formatDate";
import api from "../lib/api";
import getImgSrc from "../lib/getImgSrc";
import { safariAudioCodecs, safariVideoCodecs } from "../lib/codecs";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import ButtonGroup from "react-bootstrap/ButtonGroup";
import Dropdown from "react-bootstrap/Dropdown";
import Spinner from "react-bootstrap/Spinner";
import Badge from "react-bootstrap/Badge";
import Card from "react-bootstrap/Card";
import Image from "react-bootstrap/Image";
import Message from "../shared/Message";
import VideoExtrasModal from "../shared/VideoExtrasModal";
import {
  FaArrowLeft,
  FaStar,
  FaPlay,
  FaEye,
  FaHeart,
  FaEllipsisV,
} from "react-icons/fa";
import type { Movie } from "../types/Movie";

type MovieResponse = {
  movie: Movie;
};

export default function MovieDetailsPage() {
  const { id } = useParams();

  const navigate = useNavigate();

  const [selectedVideo, setSelectedVideo] = useState<string | null>(null);

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

  const playMovie = async () => {
    if (!data) {
      return alert("Movie data not available");
    }

    const video = document.createElement("video");
    const canPlayNativeHLS = video.canPlayType("application/vnd.apple.mpegurl");
    let copyAudio = true;
    let copyVideoCodec = true;

    if (canPlayNativeHLS) {
      if (!safariAudioCodecs.includes(data.audioList[0].codec)) {
        copyAudio = false;
      }

      if (!safariVideoCodecs.includes(data.videoList[0].codec)) {
        copyVideoCodec = false;
      }
    }

    const opts = {
      inputPath: data.filePath,
      audioStreamIndex: 0,
      audioCodec: copyAudio ? "copy" : "aac",
      audioBitRate: copyAudio ? undefined : "128k",
      audioChannels: copyAudio ? undefined : "2",
      videoStreamIndex: 0,
      videoCodec: copyVideoCodec ? "copy" : "libx264",
      videoBitrate: copyVideoCodec ? undefined : "3000k",
      videoHeight: copyVideoCodec ? undefined : "720",
      videoProfile: copyVideoCodec ? undefined : "main",
      preset: copyVideoCodec ? undefined : "fast",
    };

    alert("about to call api");

    try {
      await api.post("/ffmpeg/hls", opts);
      alert("success");
    } catch (err) {
      console.error("Transcode error:", err);
      alert("Failed to start video playback");
    }
  };

  const handleVideoSelect = (url: string) => {
    setSelectedVideo(url);
  };

  const handleCloseModal = () => {
    setSelectedVideo(null);
  };

  if (isPending) {
    return (
      <div className='min-vh-100 position-relative pt-5'>
        <Button
          variant='outline-light'
          onClick={() => navigate(-1)}
          className='position-fixed top-0 start-0 m-4 mt-5 z-3'
        >
          <FaArrowLeft className='me-2' /> Back
        </Button>
        <div className='d-flex justify-content-center align-items-center min-vh-100'>
          <Spinner
            animation='border'
            role='status'
            variant='primary'
            className='fs-3'
          >
            <span className='visually-hidden'>Loading movie details...</span>
          </Spinner>
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className='min-vh-100 position-relative pt-5'>
        <Button
          variant='outline-light'
          onClick={() => navigate(-1)}
          className='position-fixed top-0 start-0 m-4 mt-5 z-3'
        >
          <FaArrowLeft className='me-2' /> Back
        </Button>
        <Container className='py-5'>
          <Message
            title='Error Loading Movie'
            msg={error instanceof Error ? error.message : "An error occurred"}
            variant='danger'
          />
        </Container>
      </div>
    );
  }

  return (
    <main className='pt-5'>
      {/* Hero Section */}
      <header
        className='position-relative bg-dark bg-opacity-75'
        style={{
          minHeight: "80vh",
          backgroundImage: `url(${getImgSrc(data.art)})`,
          backgroundSize: "cover",
          backgroundPosition: "center",
          backgroundRepeat: "no-repeat",
          backgroundBlendMode: "overlay",
        }}
      >
        {/* Back Button */}
        <nav aria-label='Navigation'>
          <Button
            variant='outline-light'
            onClick={() => navigate(-1)}
            className='position-fixed top-0 start-0 m-4 mt-5 z-3'
          >
            <FaArrowLeft className='me-2' aria-hidden='true' />
            <span>Back</span>
          </Button>
        </nav>

        <Container className='py-5'>
          <Row className='align-items-center min-vh-75'>
            <Col md={4} lg={3}>
              <figure>
                <Image
                  src={getImgSrc(data.thumb)}
                  alt={`${data.title} poster`}
                  fluid
                  rounded
                  className='shadow h-100 object-fit-cover'
                />
              </figure>
            </Col>
            <Col md={8} lg={9}>
              <article className='text-white'>
                <h1 className='display-4 mb-2'>
                  {data.title}
                  {data.audienceRating > 0 && (
                    <Badge
                      bg='warning'
                      text='dark'
                      className='ms-3 align-middle'
                    >
                      <FaStar className='me-1' aria-hidden='true' />
                      <span
                        aria-label={`Rating: ${data.audienceRating.toFixed(
                          1
                        )} out of 10`}
                      >
                        {data.audienceRating.toFixed(1)}
                      </span>
                    </Badge>
                  )}
                </h1>

                {data.tagLine && (
                  <p className='lead mb-4 text-light fst-italic'>
                    {data.tagLine}
                  </p>
                )}

                {/* Genres */}
                {data.genres.length > 0 && (
                  <div className='mb-4'>
                    <span className='text-light opacity-75'>Genres: </span>
                    <span role='list' aria-label='Movie genres'>
                      {data.genres.map((genre, index) => (
                        <span key={genre.ID} role='listitem'>
                          {genre.title}
                          {index < data.genres.length - 1 ? ", " : ""}
                        </span>
                      ))}
                    </span>
                  </div>
                )}

                <p className='mb-4 fs-5'>{data.summary}</p>

                {/* Action Buttons */}
                <div className='mb-4' role='toolbar' aria-label='Movie actions'>
                  <ButtonGroup>
                    <Button onClick={playMovie} variant='primary' size='lg'>
                      <FaPlay className='me-2' aria-hidden='true' /> Play
                    </Button>
                    <Button variant='outline-light' size='lg'>
                      <FaEye className='me-2' aria-hidden='true' /> Mark Watched
                    </Button>
                    <Button variant='outline-light' size='lg'>
                      <FaHeart className='me-2' aria-hidden='true' /> Like
                    </Button>
                    <Dropdown as={ButtonGroup}>
                      <Dropdown.Toggle variant='outline-light' size='lg'>
                        <FaEllipsisV aria-hidden='true' />
                        <span className='visually-hidden'>More options</span>
                      </Dropdown.Toggle>
                      <Dropdown.Menu variant='dark'>
                        <Dropdown.Item href='#/action-1'>
                          Add to Playlist
                        </Dropdown.Item>
                        <Dropdown.Item href='#/action-2'>Share</Dropdown.Item>
                        <Dropdown.Item href='#/action-3'>
                          Report Issue
                        </Dropdown.Item>
                      </Dropdown.Menu>
                    </Dropdown>
                  </ButtonGroup>
                </div>

                {/* Movie Meta Info */}
                <Row className='g-4'>
                  <Col sm={6} md={4}>
                    <dt className='text-light opacity-75'>Release Date</dt>
                    <dd>{formatDate(data.releaseDate)}</dd>
                  </Col>
                  <Col sm={6} md={4}>
                    <dt className='text-light opacity-75'>Runtime</dt>
                    <dd>
                      {Math.floor(data.runTime / 60)}h {data.runTime % 60}m
                    </dd>
                  </Col>
                  <Col sm={6} md={4}>
                    <dt className='text-light opacity-75'>Rating</dt>
                    <dd>{data.contentRating}</dd>
                  </Col>
                  {data.budget > 0 && (
                    <Col sm={6} md={4}>
                      <dt className='text-light opacity-75'>Budget</dt>
                      <dd>{formatDollars(data.budget)}</dd>
                    </Col>
                  )}
                  {data.revenue > 0 && (
                    <Col sm={6} md={4}>
                      <dt className='text-light opacity-75'>Revenue</dt>
                      <dd>{formatDollars(data.revenue)}</dd>
                    </Col>
                  )}
                </Row>
              </article>
            </Col>
          </Row>
        </Container>
      </header>

      {/* Content Sections */}
      <div className='bg-dark'>
        <Container className='py-5'>
          {/* Videos and Extras Section */}
          {data.extras.length > 0 && (
            <section aria-labelledby='videos-heading' className='mb-5'>
              <h2 id='videos-heading' className='h4 mb-4 text-white'>
                Videos & Extras
              </h2>
              <Row xs={1} md={2} lg={3} className='g-4'>
                {data.extras.map(extra => (
                  <Col key={extra.ID}>
                    <Card className='h-100 bg-dark border-0 shadow'>
                      <Card.Body>
                        <h3 className='h6 mb-3 text-white'>{extra.title}</h3>
                        <div className='d-grid'>
                          <Button
                            variant='primary'
                            onClick={() => handleVideoSelect(extra.url)}
                          >
                            <FaPlay className='me-2' aria-hidden='true' /> Watch{" "}
                            {extra.kind}
                          </Button>
                        </div>
                      </Card.Body>
                    </Card>
                  </Col>
                ))}
              </Row>
            </section>
          )}

          {/* Cast Section */}
          <section aria-labelledby='cast-heading' className='mb-5'>
            <h2 id='cast-heading' className='h4 mb-4 text-white'>
              Cast
            </h2>
            <Row xs={2} md={3} lg={6} className='g-4'>
              {data.castList.map(cast => (
                <Col key={cast.ID}>
                  <Card className='h-100 bg-dark border-0 shadow'>
                    <Card.Body className='d-flex flex-column'>
                      <figure className='text-center mb-3'>
                        {cast.artist.thumb ? (
                          <Image
                            src={getImgSrc(cast.artist.thumb)}
                            alt={cast.artist.name}
                            roundedCircle
                            width={100}
                            height={100}
                            className='object-fit-cover'
                          />
                        ) : (
                          <div
                            className='rounded-circle bg-secondary d-flex align-items-center justify-content-center mx-auto'
                            style={{ width: 100, height: 100 }}
                          >
                            <i
                              className='bi bi-person fs-1 text-dark'
                              aria-hidden='true'
                            ></i>
                          </div>
                        )}
                        <figcaption>
                          <h3 className='h6 text-center mb-1 text-white'>
                            {cast.artist.name}
                          </h3>
                          <p className='text-light opacity-75 text-center small mb-0'>
                            {cast.character}
                          </p>
                        </figcaption>
                      </figure>
                    </Card.Body>
                  </Card>
                </Col>
              ))}
            </Row>
          </section>

          {/* Crew Section */}
          <section aria-labelledby='crew-heading' className='mb-5'>
            <h2 id='crew-heading' className='h4 mb-4 text-white'>
              Crew
            </h2>
            <Row xs={2} md={3} lg={6} className='g-4'>
              {data.crewList.map(crew => (
                <Col key={crew.ID}>
                  <Card className='h-100 bg-dark border-0 shadow'>
                    <Card.Body className='d-flex flex-column'>
                      <figure className='text-center mb-3'>
                        {crew.artist.thumb ? (
                          <Image
                            src={getImgSrc(crew.artist.thumb)}
                            alt={crew.artist.name}
                            roundedCircle
                            width={100}
                            height={100}
                            className='object-fit-cover'
                          />
                        ) : (
                          <div
                            className='rounded-circle bg-secondary d-flex align-items-center justify-content-center mx-auto'
                            style={{ width: 100, height: 100 }}
                          >
                            <i
                              className='bi bi-person fs-1 text-dark'
                              aria-hidden='true'
                            ></i>
                          </div>
                        )}
                        <figcaption>
                          <h3 className='h6 text-center mb-1 text-white'>
                            {crew.artist.name}
                          </h3>
                          <p className='text-light opacity-75 text-center small mb-0'>
                            {crew.job}
                          </p>
                        </figcaption>
                      </figure>
                    </Card.Body>
                  </Card>
                </Col>
              ))}
            </Row>
          </section>

          {/* Studios Section */}
          {data.studios.length > 0 && (
            <section aria-labelledby='studios-heading'>
              <h2 id='studios-heading' className='h4 mb-4 text-white'>
                Studios
              </h2>
              <Row xs={2} md={3} lg={6} className='g-4'>
                {data.studios.map(studio => (
                  <Col key={studio.ID}>
                    <Card className='h-100 bg-dark border-0 shadow'>
                      <Card.Body className='d-flex flex-column justify-content-center'>
                        <figure className='text-center mb-3'>
                          {studio.logo ? (
                            <Image
                              src={getImgSrc(studio.logo)}
                              alt={studio.name}
                              className='w-100 object-fit-contain'
                              height={60}
                            />
                          ) : (
                            <div className='bg-secondary d-flex align-items-center justify-content-center h-100'>
                              <i
                                className='bi bi-building fs-1 text-dark'
                                aria-hidden='true'
                              ></i>
                            </div>
                          )}
                          <figcaption>
                            <h3 className='h6 text-center mb-0 text-white'>
                              {studio.name}
                            </h3>
                          </figcaption>
                        </figure>
                      </Card.Body>
                    </Card>
                  </Col>
                ))}
              </Row>
            </section>
          )}
        </Container>
      </div>

      {/* Video Modal */}
      <VideoExtrasModal
        show={!!selectedVideo}
        onHide={handleCloseModal}
        videoUrl={selectedVideo}
      />
    </main>
  );
}
