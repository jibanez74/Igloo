const Trailer = {
  title: {
    type: String,
    trim: true,
    default: "unknown",
  },

  url: {
    type: String,
    required: [true, "url is required"],
    trim: true,
  },
};

export default Trailer;
