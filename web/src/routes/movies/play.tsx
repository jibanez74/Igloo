import { useState } from "react";
import { createFileRoute, useSearch } from "@tanstack/react-router";
import z from "zod";
import ReactPlayer from "react-player";
import { FaPlay } from "react-icons/fa";

const searchSchema = z.object({
  container: z.string(),
  filePath: z.string(),
  contentType: z.string(),
  thumb: z.string().optional(),
});

export const Route = createFileRoute("/movies/play")({
  component: PlayMoviePage,
  validateSearch: searchSchema.parse,
});

function PlayMoviePage() {
  const search = useSearch({ from: "/movies/play" });

  const [isPlaying, setIsPlaying] = useState(true);

  const url = `/api/v1/streaming/video?filePath=${encodeURIComponent(search.filePath)}&contentType=${encodeURIComponent(search.contentType)}`;

  return (
    <div className='p-4 rounded-lg shadow-lg max-w-full'>
      <ReactPlayer
        url={url}
        controls={true}
        playing={isPlaying}
        onPause={() => setIsPlaying(false)}
        onPlay={() => setIsPlaying(true)}
        loop={false} // Don't loop the video
        pip={true} // Enable Picture-in-Picture mode
        playIcon={<FaPlay />}
        muted={true}
        width='100%'
        height='100%'
      />
    </div>
  );
}
