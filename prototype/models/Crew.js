import mongoose from "mongoose";

const Crew = {
  artist: {
    type: [mongoose.Schema.Types.ObjectId],
    ref: "artist",
    required: [true, "artist is required"],
  },

  job: {
    type: String,
    default: "unknown",
    trim: true,
  },

  department: {
    type: String,
    default: "unknown",
    trim: true,
  },
};

export default Crew;
