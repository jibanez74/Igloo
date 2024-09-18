import mongoose from "mongoose";

const Cast = {
  artist: {
    type: [mongoose.Schema.Types.ObjectId],
    ref: "artist",
    required: [true, "artist is required"],
  },

  character: {
    type: String,
    default: "unknown",
    trim: true,
  },

  order: Number,
};

export default Cast;
