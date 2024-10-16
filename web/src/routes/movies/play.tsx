import { useState } from "react";
import { createFileRoute, useSearch } from "@tanstack/react-router";
import z from "zod";
import ReactPlayer from "react-player";
import { FaPlay } from "react-icons/fa";

const searchSchema = z.object({
  transcode: z.boolean(),
  container: z.string(),
  filePath: z.string(),
  contentType: z.string(),
  thumb: z.string().optional(),
  videoCodec: z.string().optional(),
  videoBitRate: z.string().optional(),
  videoHeight: z.number().optional(),
  audioCodec: z.string().optional(),
  audioBitRate: z.string().optional(),
  audioChannels: z.number().optional(),
});

export const Route = createFileRoute("/movies/play")({
  component: PlayMoviePage,
  validateSearch: searchSchema.parse,
});

function PlayMoviePage() {
  const search = useSearch({ from: "/movies/play" });

  const [isPlaying, setIsPlaying] = useState(true);

  const filePath = encodeURIComponent(search.filePath);
  const contentType = encodeURIComponent(search.contentType);

  let url = `/api/v1/streaming/video?filePath=${filePath}&contentType=${contentType}`;

  if (search.transcode) {
    url = `/api/v1/streaming/video/transcode?container=${search.container}&filePath=${filePath}&contentType=${contentType}&videoHeight=${search.videoHeight}&videoCodec=${search.videoCodec}&videoBitRate=${search.videoBitRate}&audioCodec=${search.audioCodec}&audioBitRate=${search.audioBitRate}&audioChannels=${search.audioChannels}`;
  }

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
