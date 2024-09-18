import mongoose from "mongoose";

const ArtistSchema = new mongoose.Schema(
  {
    name: {
      type: String,
      required: [true, "name is required"],
      maxLength: [60, "name must not be longer than 60 characters long"],
      index: true,
    },

    originalName: {
      type: String,
      trim: true,
      maxLength: [60, "name must not be longer than 60 characters long"],
    },

    thumb: {
      type: String,
      trim: true,
      default: "no_thumb.png",
    },

    knownFor: {
      type: String,
      default: "unknown",
      trim: true,
    },
  },
  {
    timestamps: true,
  }
);

const Artist = mongoose.model("artist", ArtistSchema);

export default Artist;
