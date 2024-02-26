const path = require("path");
const fs = require("fs");
const plex = require("../config/plex");
const { v4 } = require("uuid");

async function saveImage(apiPath, filePath) {
  try {
    const { data } = await plex.get(apiPath, {
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

module.exports = saveImage;
