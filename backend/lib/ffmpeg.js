import ffmpeg from "fluent-ffmpeg";
import path from "path";

const ffmpegPath = path.join(
  process.cwd(),
  "bin",
  process.env.FFMPEG_DIR,
  "ffmpeg"
);

const ffprobePath = path.join(
  process.cwd(),
  "bin",
  process.env.FFMPEG_DIR,
  "ffprobe"
);

ffmpeg.setFfmpegPath(ffmpegPath);
ffmpeg.setFfprobePath(ffprobePath);

export const getVideoMetadata = filePath => {
  return new Promise((resolve, reject) => {
    if (filePath === "") {
      reject(new Error("file path is empty"));
    }

    ffmpeg.ffprobe(filePath, (err, metadata) => {
      if (err) reject(err);

      const mediaData = {
        duration: metadata.format.duration,
        format: metadata.format.format_name,
        size: metadata.format.size,
        bitRate: metadata.format.bit_rate,
      };

      mediaData.videos = metadata.streams
        .filter(v => v.codec_type === "video")
        .map(v => ({
          title: v.codec_long_name,
          index: v.index,
          codec: v.codec_name,
          profile: v.profile,
          width: v.width,
          height: v.height,
          codedWidth: v.coded_width,
          codedHeight: v.coded_height,
          aspectRatio: v.display_aspect_ratio,
          bitDepth: v.pix_fmt,
          level: v.level,
          colorTransfer: v.color_transfer,
          colorPrimaries: v.color_primaries,
          colorSpace: v.color_space,
          frameRate: v.r_frame_rate,
          avgFrameRate: v.avg_frame_rate,
          numberOfFrames: v.tags?.NUMBER_OF_FRAMES,
          numberOfBytes: v.tags?.NUMBER_OF_BYTES,
          bps: v.tags?.BPS,
        }));

      mediaData.audio = metadata.streams
        .filter(a => a.codec_type === "audio")
        .map(a => ({
          title: a.tags?.title,
          index: a.index,
          profile: a.profile,
          codec: a.codec_name,
          channels: a.channels,
          channelLayout: a.channel_layout,
          language: a.tags?.language,
          bps: a.tags?.BPS,
        }));

      mediaData.subtitles = metadata.streams
        .filter(sub => sub.codec_type === "subtitle")
        .map(sub => ({
          title: sub.tags?.title,
          index: sub.index,
          codec: sub.codec_name,
          language: sub.tags?.language,
        }));

      resolve(mediaData);
    });
  });
};
