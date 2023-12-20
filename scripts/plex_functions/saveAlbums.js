const path = require("path");
const fs = require("fs");
const api = require("../config/api");
const plex = require("../config/plex");
const saveMoods = require("../utils/saveMoods");
const saveGenres = require("../utils/sav∫eGenres");
const saveImage = require("../utils/saveImage");

async function saveAlbums() {
  let plexAlbums = null;
  const errors = [];

  try {
    const res = await plex.get("/library/sections/11/albums");

    plexAlbums = data.MediaContainer.Metadata;
  } catch (err) {
    console.error("Unable to fetch albums from plex", err);
    process.exit(1);
  }

  if (!plexAlbum || plexAlbums.length === 0) {
    console.log("No album were fetch from plex");
    process.exit(0);
  }

  for (const a of plexAlbums) {
    let exist = false;

    try {
      const res = await api.get(`/album/name/${name}`);

      exist = true;
    } catch (err) {
      if (err.response && err.response.status !== 404) {
        console.error(err);
        const errorFilePath = path.join(__dirname, "errors.json");
        fs.writeFileSync(errorFilePath, JSON.stringify(err));
      } else if (err.response && err.response.status === 404) {
        exist = false;
      } else {
        console.error(err);
        process.exit(1);
      }
    }

    if (exist) {
      continue;
    }

    const album = {
      title: a.title,
      numberOfTracks: a.leafCount,
      studio: a.studio ? a.studio : undefined,
      year: a.year ? a.year : undefined,
    };

    if (a.parentTitle) {
      try {
        const res = await api.get(`/musician/name/${a.parentTitle}`);

        album.musicians = [res.data.data];
      } catch (err) {
        const errorFilePath = path.join(__dirname, "errors.json");
        fs.writeFileSync(errorFilePath, JSON.stringify(err));

        process.exit(3);
      }
    }

    if (a.Genre && Array.isArray(a.Genre)) {
      album.genres = await saveGenres(a.Genre);
    }

    if (a.thumb) {
      album.thumb = await saveImage(a.thumb, "public/images/albums/thumb");
    }

    if (a.art) {
      album.art = await saveImage(a.art, "public/images/albums/art");
    }

    if (a.originallyAvailableAt) {
      const date = new Date(a.originallyAvailableAt);
      album.releaseDate = date.toISOString();
    }
  }

  console.log("Finished saving album data");

  process.exit(0);
}
