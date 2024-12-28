import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import Container from "react-bootstrap/Container";
import Button from "react-bootstrap/Button";
import Message from "../shared/Message";
import HlsPlayer from "../shared/HlsPlayer";
import { FaArrowLeft } from "react-icons/fa";

export default function PlayMovie() {
  const navigate = useNavigate();

  const [searchParams] = useSearchParams();
  const fileName = searchParams.get("file_name");
  const nativeHls = searchParams.get("native_hls") === "true";
  const title = searchParams.get("title");
  const poster = searchParams.get("poster");

  const [error, setError] = useState<string | null>(null);

  if (!fileName) {
    return (
      <Container className='py-5'>
        <Message title='Error' msg='No video file specified' variant='danger' />
      </Container>
    );
  }

  const handleError = (error: Error) => {
    console.error("Playback Error:", error);
    setError(error.message);
  };

  const handleEnded = () => {
    navigate(-1);
  };

  return (
    <main className='min-vh-100 bg-dark'>
      {/* Back Button */}
      <Button
        variant='outline-light'
        onClick={() => navigate(-1)}
        className='position-fixed top-0 start-0 m-4 z-3'
      >
        <FaArrowLeft className='me-2' /> Back
      </Button>

      {/* Error Message */}
      {error && (
        <Container className='py-5'>
          <Message
            title='Playback Error'
            msg={error}
            variant='danger'
            duration={5000}
          />
        </Container>
      )}

      {/* Video Player */}
      {!error && (
        <div className='d-flex align-items-center justify-content-center min-vh-100'>
          <Container fluid className='px-0'>
            <HlsPlayer
              url={`/api/v1/movies/stream/${fileName}`}
              title={title || undefined}
              poster={poster || undefined}
              useHlsJs={!nativeHls}
              onError={handleError}
              onEnded={handleEnded}
              onProgress={state => {
                // You can add progress tracking here
                console.log("Progress:", state.played);
              }}
            />
          </Container>
        </div>
      )}
    </main>
  );
}
