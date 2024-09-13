const AudioStream = {
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

  codec: {
    type: String,
    required: [true, "codec is required"],
    trim: true,
  },

  channels: {
    type: Number,
    required: [true, "channels is required"],
  },

  channelLayout: {
    type: Number,
    required: [true, "channel layout is required"],
  },

  bps: {
    type: Number,
    required: [true, "bps is required"],
  },

  language: {
    type: String,
    default: "unknown",
    trim: true,
  },
};

export default AudioStream;
