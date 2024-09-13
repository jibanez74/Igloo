import mongoose from "mongoose";

const StudioSchema = new mongoose.Schema({
  name: {
    type: String,
    required: [true, "name is required"],
    maxLength: [80, "name must not be longer than 80 characters"],
    trim: true,
  },

  country: {
    type: String,
    default: "unknown",
    trim: true,
    maxLength: [20, "country must not be longer than 20 characters"],
  },
});

const Studio = mongoose.model("studio", StudioSchema);

export default Studio;
