import mongoose from "mongoose";

const UserSchema = new mongoose.Schema(
  {
    name: {
      type: String,
      required: [true, "name is required"],
      minLength: [2, "name must be at least 2 characters long"],
      maxLength: [60, "name must not be longer than 60 characters"],
      trim: true,
    },

    username: {
      type: String,
      required: [true, "username is required"],
      trim: true,
      unique: true,
      index: true,
    },

    eemail: {
      type: String,
      required: [true, "email is required"],
      trim: true,
      unique: true,
      index: true,
    },

    password: {
      type: String,
      required: [true, "password is required"],
      minLength: [9, "password must be at least 9 characters long"],
      maxLength: [128, "password must not be longer than 128 characters"],
    },
  },
  {
    timestamps: true,
  }
);

const User = mongoose.model("user", UserSchema);

export default User;
