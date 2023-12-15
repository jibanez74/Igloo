const saveMusicians = require("./plex_functions/saveMusicians");

main();

async function main() {
  await saveMusicians();
}
