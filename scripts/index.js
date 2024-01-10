const saveMusicians = require("./plex_functions/saveMusicians");
const saveAlbums = require("./plex_functions/saveAlbums");
main();

async function main() {
  //  await saveMusicians();
  await saveAlbums();
}
