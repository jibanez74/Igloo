const VideoStream = {
  title: {
    type: String,
    required: [true, "title is required"],
    trim: true,
  },

  index: {
    type: Number,
    required: [true, "index is required"],
  },

  profile: {
    type: String,
    required: [true, "profile is required"],
  },

  bitDepth: {
    type: String,
    required: [true, "bit depth is required"],
  },

  codec: {
    type: String,
    required: [true, "codec is required"],
  },

  width: {
    type: Number,
    required: [true, "width is required"],
  },

  height: {
    type: Number,
    required: [true, "height is required"],
  },

  codedWidth: {
    type: String,
    required: [true, "coded width is required"],
  },

  codedHeight: {
    type: String,
    required: [true, "coded height is required"],
  },

  aspectRatio: {
    type: String,
    default: "unknown",
    trim: true,
  },

  colorSpace: {
    type: String,
    default: "unknown",
    trim: true,
  },

  colorPrimaries: {
    type: String,
    default: "unknown",
    trim: true,
  },

  frameRate: {
    type: Number,
    default: 0,
  },

  avgFrameRate: {
    type: Number,
    default: 0,
  },

  bps: {
    type: Number,
    required: [true, "bps is required"],
  },

  numberOfFrames: {
    type: Number,
    required: [true, "number of frames is required"],
  },

  numberOfBytes: {
    type: Number,
    required: [true, "number of bytes is required"],
  },
};

export default VideoStream;
