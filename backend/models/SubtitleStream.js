const SubtitleStream = {
  title: {
    type: String,
    required: [true, "title is required"],
    trim: true,
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
