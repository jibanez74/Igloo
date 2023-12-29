const api = require("../config/api");
const fs = require("fs");
const path = require("path");

async function saveGenres(genres) {
  const genreList = [];

  if (!genres) {
    const res = await api.post("/music-genre", { tag: "unknown" });

    genreList.push(res.data);

    return genreList;
  }

  for (const g of genres) {
    try {
      const res = await api.post("/music-genre", { tag: g.tag });

      genreList.push(res.data);
    } catch (err) {
      console.log("error saving genre", err);
      const errorFilePath = path.join(__dirname, "errors.json");
      fs.writeFileSync(errorFilePath, JSON.stringify(err));

      process.exit(3);
    }
  }

  return genreList;
}

module.exports = saveGenres;
