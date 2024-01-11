const saveMusicians = require("./plex_functions/saveMusicians");
const saveAlbums = require("./plex_functions/saveAlbums");
const fs = require("fs");
const path = require("path");
const api = require("./config/api");

main();

async function main() {
  //   await saveMusicians();

  const { data } = await api.get("/musicians/all");

  // write the data to a json file
  fs.writeFileSync(path.join(__dirname, "data.json"), JSON.stringify(data));
}
