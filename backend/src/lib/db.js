import mongoose from "mongoose";

const connectToDB = async () => {
  try {
    mongoose.set("strictQuery", true);

    await mongoose.connect(process.env.MONGO_URI);

    console.log(`Connected to mongo using ${process.env.MONGO_URI}`);
  } catch (err) {
    console.error(err);
    process.exit(1);
  }
};

export default connectToDB;
