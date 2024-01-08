const path = require("path");
const fs = require("fs");
const api = require("../config/api");
const plex = require("../config/plex");
const saveGenres = require("../utils/saveGenres");
const saveImage = require("../utils/saveImage");

async function saveAlbums() {
  let plexAlbums = null;
  const errors = [];

  try {
    const res = await plex.get("/library/sections/11/albums");

    plexAlbums = res.data.MediaContainer.Metadata;
  } catch (err) {
    console.error("Unable to fetch albums from plex", err);
    process.exit(1);
  }

  if (!plexAlbums || plexAlbums.length === 0) {
    console.log("No album were fetch from plex");
    process.exit(0);
  }

  for (const a of plexAlbums) {
    let exist = false;

    try {
      const res = await api.get(`/album/title/${a.title}`);

      exist = true;
    } catch (err) {
      if (err.response && err.response.status !== 404) {
        console.error(err);
        const errorFilePath = path.join(__dirname, "errors.json");
        fs.writeFileSync(errorFilePath, JSON.stringify(err));
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
      summary: a.summary ? a.summary : undefined,
    };

    if (a.parentTitle) {
      try {
        const res = await api.get(`/musician/name/${a.parentTitle}`);

        album.musicians = [res.data];
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

    try {
      await api.post("/album", album);
    } catch (err) {
      console.log("unable to save album", err);
      process.exit(4);
    }
  }

  console.log("Finished saving album data");

  process.exit(0);
}

module.exports = saveAlbums;
