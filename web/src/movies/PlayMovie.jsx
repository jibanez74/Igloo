import { useParams } from "react-router-dom";
import VideoPlayer from "../shared/VideoPlayer";

export default function PlayMoviePage() {
  const { id } = useParams();

  const src = `/api/v1/movie/stream/${id}`;

  return (
    <div className='container mx-auto'>
      <VideoPlayer src={src} />
    </div>
  );
}
