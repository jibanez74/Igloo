const path = require("path");
const fs = require("fs");
const { v4 } = require("uuid");
const axios = require("axios");
const dotenv = require("dotenv");

dotenv.config({ path: "./config.env" });

const plex = axios.create({
  baseURL: "http://100.107.177.6:32400",
  headers: { "X-Plex-Token": process.env.PLEX_TOKEN },
});

const api = axios.create({
  baseURL: "http://localhost:8080/api/v1",
});

async function saveMoods(moods) {
  try {
    const moodList = [];

    for (const m of moods) {
      const exist = await Mood.findOne({ tag: m.tag });

      if (exist) {
        moodList.push(exist._id);
      } else {
        const newMood = await Mood.create({ tag: m.tag });

        moodList.push(newMood);
      }
    }

    return moodList;
  } catch (err) {
    throw err;
  }
}

async function saveTracks(path, album, artists, genres) {
  try {
    const { data: tracks } = await plex.get(path);

    for (const x of tracks.MediaContainer.Metadata) {
      const { data: song } = await plex.get(x.key);
      const t = song.MediaContainer.Metadata[0];

      const track = {
        title: t.title,
        remoteFile: t.Media[0].Part[0].file,
        duration: t.duration,
        index: t.index,
        bitrate: t.Media[0].bitrate,
        container: t.Media[0].container,
        size: t.Media[0].Part[0].size,
        audioChannels: t.Media[0].saveAlbums,
        codec: t.Media[0].audioCodec,
        genres,
        album: album._id,
        artists,
      };

      if (t.parentYear) {
        track.year = t.parentYear;
      }

      if (t.Mood && Array.isArray(t.Mood)) {
        track.moods = await saveMoods(t.Mood);
      }

      await Track.create(track);
    }
  } catch (err) {
    throw err;
  }
}

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
}

