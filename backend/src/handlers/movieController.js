import path from "path";
import Movie from "../models/Movie";
import { ffmpegInstance as ffmpeg } from "../lib/ffmpeg";
import { Readable } from "stream";

export const getMovieCount = async c => {
  const count = await Movie.countDocuments();

  return c.json({ count }, 200);
};

export const getLatestMovies = async c => {
  const movies = await Movie.find({}, { _id: 1, title: 1, thumb: 1, year: 1 })
    .sort({ createdAt: -1 })
    .limit(12);

  return c.json({ movies }, 200);
};

export const getMovieByID = async c => {
  const id = c.req.param("id");

  if (!id) {
    return c.json({ error: "no id provided" }, 400);
  }

  const movie = await Movie.findById(id)
    .populate("genres")
    .populate("studios")
    .populate({
      path: "crewList",
      populate: {
        path: "artist",
        model: "artist",
      },
    })
    .populate({
      path: "castList",
      populate: {
        path: "artist",
        model: "artist",
      },
    });

  if (!movie) {
    return c.json({ error: `unable to find movie with id of ${id}` }, 404);
  }

  return c.json({ movie }, 200);
};

export const getMovies = async c => {
  const movies = await Movie.find(
    {},
    { _id: 1, title: 1, thumb: 1, year: 1 }
  ).sort({ title: 1 });

  return c.json({ movies }, 200);
};

export const streamMovie = async c => {
  const id = c.req.param("id");
  const transcode = c.req.query("transcode");

  if (!id) {
    return c.json({ error: "no id provided" }, 400);
  }

  const movie = await Movie.findById(id);

  if (!movie) {
    return c.json({ error: `unable to find movie with id of ${id}` }, 404);
  }

  const videoFile = Bun.file(movie.filePath);

  const exists = await videoFile.exists();
  if (!exists) {
    return c.json(
      { error: `unable to find a video file for movie with id ${id}` },
      404
    );
  }

  if (transcode === "yes") {
    const videoStream = await videoFile.stream();

    return new Response(
      new ReadableStream({
        async start(controller) {
          const inputStream = Readable.from(videoStream);

          const cmd = ffmpeg(inputStream)
            .audioChannels(2)
            .audioBitrate("196k")
            .audioCodec("aac")
            .videoCodec("h264_nvenc")
            .outputOptions([
              "-movflags frag_keyframe+empty_moov",
              "-preset ultrafast",
              "-crf 18",
            ])
            .format("mp4")
            .on("error", (err, stdout, stderr) => {
              console.error(`FFmpeg error: ${err.message}`);
              console.error(`FFmpeg stderr: ${stderr}`);
              controller.error(err);
            })
            .on("end", () => {
              console.log("FFmpeg command ended");
              controller.close();
            });

          const outputStream = cmd.pipe();

          outputStream.on("data", chunk => {
            controller.enqueue(chunk);
          });

          outputStream.on("end", () => {
            controller.close();
          });
        },
      }),
      {
        headers: {
          "Content-Type": "video/mp4",
        },
      }
    );
  } else {
    const fileSize = videoFile.size;
    const range = c.req.header("range");

    if (range) {
      const parts = range.replace(/bytes=/, "").split("-");
      const start = parseInt(parts[0], 10);
      const end = parts[1] ? parseInt(parts[1], 10) : fileSize - 1;
      const chunkSize = end - start + 1;

      c.status(206);
      c.header("Content-Range", `bytes ${start}-${end}/${fileSize}`);
      c.header("Accept-Ranges", "bytes");
      c.header("Content-Length", chunkSize.toString());
      c.header("Content-Type", movie.contentType);

      return c.body(videoFile.slice(start, end + 1));
    }

    c.header("Content-Length", fileSize.toString());
    c.header("Content-Type", movie.contentType);

    return c.body(videoFile);
  }
};
