import { db as mongoose } from "../lib/db";

const GenreSchema = new mongoose.Schema({
  tag: {
    type: String,
    required: [true, "tag is required"],
    trim: true,
    unique: true,
    index: true,
  },

  genreType: {
    type: String,
    required: [true, "genre type is required"],
    enum: ["music", "movie"],
  },
});

const Genre = mongoose.model("genre", GenreSchema);

export default Genre;
