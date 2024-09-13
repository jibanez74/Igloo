import VideoStream from "./VideoStream";
import SubtitleStream from "./SubtitleStream";
import AudioStream from "./AudioStream";

const MediaContainer = {
  duration: {
    type: Number,
    required: [true, "duration is required"],
  },

  format: {
    type: String,
    required: [true, "container is required"],
    trim: true,
  },

  size: {
    type: Number,
    required: [true, "size is required"],
  },

  bitRate: {
    type: Number,
    required: [true, "bit rate is required"],
  },

  videos: {
    type: [VideoStream],
    required: [true, "video streams myst not be empty"],
  },

  audio: {
    type: [AudioStream],
    default: [],
  },

  subtitles: {
    type: [SubtitleStream],
    default: [],
  },
};

export default MediaContainer;
