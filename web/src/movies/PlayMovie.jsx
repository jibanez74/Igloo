import { useSearchParams } from "react-router-dom";
import VideoPlayer from "../shared/VideoPlayer";

export default function PlayMovie() {
  const params = useSearchParams();
  const pid = params.get("pid");
  const filePath = params.get("file_path");

  return (
    <div className='container mx-auto'>
      <VideoPlayer
        options={{
          autoplay: true,
          controls: true,
          fluid: true,
          responsive: true,
          sources: [
            {
              src: "/api/v1/play",
              type: "application/x-mpegURL",
            },
          ],
        }}
      />
    </div>
  );
}
