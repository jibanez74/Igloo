const path = require("path");
const fs = require("fs");
const axios = require("axios");
const { v4 } = require("uuid");

const api = axios.create({
  baseURL: "https://image.tmdb.org/t/p/original",
});

async function saveTmdbImage(path, filePath) {
  try {
    const { data } = await api.get(path, {
      responseType: "arraybuffer",
    });

    const fileNameAndPath = `${filePath}/${v4()}.png`;

    if (!fs.existsSync(filePath)) {
      fs.mkdirSync(filePath, { recursive: true });
    }

    fs.writeFileSync(fileNameAndPath, data);

    return fileNameAndPath;
  } catch (err) {
    console.error(err);
    process.exit(2);
  }
}

module.exports = saveTmdbImage;
