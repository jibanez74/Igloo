import { createFileRoute, useSearch } from "@tanstack/react-router";
import z from "zod";
import ReactPlayer from "react-player";
import { FaPlay } from "react-icons/fa";
import { v4 } from "uuid";
import { useEffect } from "react";

const searchSchema = z.object({
  id: z.number(),
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
  const fileID = v4();

  const search = useSearch({ from: "/movies/play" });

  const url = `/api/v1/auth/stream/video/simple-transcode/${search.id}?uuid=${fileID}`;

  useEffect(() => {
    return () => {
      fetch(`/api/v1/auth/stream/video/remove/${fileID}`, {
        method: "delete",
        credentials: "include",
      })
        .then(() => console.log("video file was deleted"))
        .catch(() => alert("unable to delete video file"));
    };
  }, [fileID]);

  return (
    <div className='p-4 rounded-lg shadow-lg max-w-full'>
      <ReactPlayer
        url={url}
        controls={true}
        playIcon={<FaPlay />}
        width='100%'
        height='100%'
      />
    </div>
  );
}
