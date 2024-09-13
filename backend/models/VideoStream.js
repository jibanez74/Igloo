const VideoStream = {
  title: {
    type: String,
    trim: true,
    default: "unknown",
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

  level: {
    type: Number,
    required: [true, "level is required"],
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

  colorTransfer: {
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
    type: String,
    default: 0,
  },

  avgFrameRate: {
    type: String,
    default: 0,
  },

  bps: {
    type: Number,
    default: 0,
  },

  numberOfFrames: {
    type: Number,
    default: 0,
  },

  numberOfBytes: {
    type: Number,
    default: 0,
  },
};

export default VideoStream;
