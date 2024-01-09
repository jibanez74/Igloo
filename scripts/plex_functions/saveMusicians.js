const fs = require("fs");
const path = require("path");
const api = require("../config/api");
const plex = require("../config/plex");
const saveGenres = require("../utils/saveGenres");
const saveImage = require("../utils/saveImage");

async function saveMusicians() {
  console.log("starting process to save musicians");

  let plexArtistData = null;
  const errors = [];

  try {
    const { data } = await plex.get("/library/sections/11/all");

    plexArtistData = data.MediaContainer.Metadata;
  } catch (err) {
    console.error(`Unable to fetch artists from plex: \n ${err}`);
    process.exit(1);
  }

  if (!plexArtistData || plexArtistData.length === 0) {
    console.log("No artists found in plex");
    process.exit(0);
  }

  for (const artist of plexArtistData) {
    const exist = await checkArtist(artist.title);

    if (exist) {
      continue;
    }

    const a = {
      name: artist.title,
      summary: artist.summary ? artist.summary : "",
    };

    if (artist.thumb) {
      a.thumb = await saveImage(artist.thumb, "public/images/musicians/thumb");
    }

    if (artist.art) {
      a.art = await saveImage(artist.art, "public/images/musicians/art");
    }

    a.genres = await saveGenres(artist.Genre);

    try {
      await api.post("/musician", a);
    } catch (err) {
      console.error(err.response.data);
      // write the err to a json file
      const errorFilePath = path.join(__dirname, "errors.json");
      fs.writeFileSync(errorFilePath, JSON.stringify(err));

      process.exit(5);
    }
  }

  if (errors.length > 0) {
    const errorFilePath = path.join(__dirname, "errors.json");
    fs.writeFileSync(errorFilePath, JSON.stringify(errors));
    console.log(
      "Some errors occurred during the process. Errors were written to the errors.json file."
    );
  }

  console.log("Finished with saving musicians");

  process.exit(0);
}

async function checkArtist(title) {
  let exist = false;

  try {
    const res = await api.get(`/api/v1/musician/name/${title}`);

    exist = true;
  } catch (err) {
    if (err.response.status !== 404) {
      console.error(err.response.status);
      process.exist(3);
    }
  }

  return true;
}

module.exports = saveMusicians;
