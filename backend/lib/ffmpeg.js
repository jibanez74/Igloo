import ffmpeg from "fluent-ffmpeg";
import path from "path";

const ffmpegPath = path.join(process.cwd(), "bin", "ffmpeg");
const ffprobePath = path.join(process.cwd(), "bin", "ffprobe");

ffmpeg.setFfmpegPath(ffmpegPath);
ffmpeg.setFfprobePath(ffprobePath);
