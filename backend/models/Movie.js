import path from "path";
import fs from "fs";
import mongoose from "mongoose";
import Genre from "./Genre";
import Studio from "./Studio";
import Trailer from "./Trailer";
import Artist from "./Artist";
import Crew from "./Crew";
import Cast from "./Cast";
import MediaContainer from "./MediaContainer";

const MovieSchema = new mongoose.Schema(
  {
    title: {
      type: String,
      required: [true, "title is required"],
      trim: true,
      index: true,
    },

    filePath: {
      type: String,
      required: [true, "file path is required"],
      trim: true,
      unique: true,
    },

    container: {
      type: String,
      default: "unknown",
    },

    resolution: {
      type: String,
      default: "unknown",
      trim: true,
      enum: ["unknown", "480p", "720p", "1080p", "2160p"],
    },

    runTime: {
      type: Number,
      required: [true, "runtime is required"],
    },

    tagline: {
      type: String,
      default: "unknown",
    },

    summary: {
      type: String,
      default: "unknown",
    },

    thumb: {
      type: String,
      default: "no_thumb.png",
      trim: true,
    },

    art: {
      type: String,
      default: "no_art.png",
      trim: true,
    },

    tmdbID: {
      type: String,
      default: "unknown",
      index: true,
    },

    imdbID: {
      type: String,
      default: "unknown",
    },

    year: Number,

    releaseDate: Date,

    Budget: Number,

    revenue: Number,

    contentRating: {
      type: String,
      default: "unknown",
    },

    criticRating: {
      type: Number,
      default: 0,
    },

    audienceRating: {
      type: Number,
      default: 0,
    },

    spokenLanguages: {
      type: String,
      default: "unknown",
    },

    trailers: {
      type: [Trailer],
      default: [],
    },

    genres: {
      type: [mongoose.Schema.Types.ObjectId],
      ref: "genre",
      default: [],
    },

    studios: {
      type: [mongoose.Schema.Types.ObjectId],
      ref: "studio",
      default: [],
    },

    castList: {
      type: [Cast],
      default: [],
    },

    crewList: {
      type: [Crew],
      default: [],
    },

    mediaContainer: MediaContainer,
  },
  {
    timestamps: true,
  }
);

const Movie = mongoose.model("movie", MovieSchema);

export default Movie;
