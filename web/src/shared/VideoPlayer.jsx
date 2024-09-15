import { useRef } from "react";
import { useParams } from "react-router-dom";
import ReactPlayer from "react-player";
import { FaPlay } from "react-icons/fa";

export default function VideoPlayer() {
  const videoRef = useRef();
  const { id } = useParams();

  const url = `/api/v1/movie/stream/${id}`;

  return (
    <div className='container mx-auto'>
      <ReactPlayer
        controls={true}
        url={url}
        playIcon={<FaPlay />}
        height={100}
        width={100}
        ref={videoRef}
      />
    </div>
  );
}
