const SubtitleStream = {
  title: {
    type: String,
    trim: true,
    default: "unknown",
  },

  index: {
    type: Number,
    required: [true, "index is required"],
  },

  codec: {
    type: String,
    required: [true, "codec is required"],
    trim: true,
  },

  language: {
    type: String,
    default: "unknown",
    trim: true,
  },
};

export default SubtitleStream;
