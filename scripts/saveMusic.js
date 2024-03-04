// file for moving music data from plex to igloo data base

const fs = require("fs");
const path = require("path");
const api = require("./config/api");
const saveImage = require("./utils/savePlexImage");
const plex = require("./config/plex");

main();

async function main() {
  await saveMusicians();
  await saveAlbums();
}

async function saveTracks(key, album) {
  try {
    const res = await plex.get(key);

    for (const x of res.data.MediaContainer.Metadata) {
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
        album: album.ID,
        musicians: album.musicians,
        genres: album.genres,
        album,
      };

      if (t.parentYear) {
        track.year = t.parentYear;
      }

      if (t.Mood && Array.isArray(t.Mood)) {
        track.moods = await saveMoods(t.Mood);
      }

      await api.post("/track", track);
    }

    return true;
  } catch (err) {
    throw err;
  }
}

async function saveAlbums() {
  try {
    const res = await plex.get("/library/sections/11/albums");

    console.log(
      `Got ${res.data.MediaContainer.size} albums from plex.  About to start loop to save data into igloo.`
    );

    for (const a of res.data.MediaContainer.Metadata) {
      const album = {
        title: a.title,
        numberOfTracks: a.leafCount,
        studio: a.studio ? a.studio : undefined,
        year: a.year ? a.year : undefined,
        summary: a.summary ? a.summary : undefined,
      };

      if (a.parentTitle) {
        const res = await api.post("/musician/name", { name: a.parentTitle });

        album.musicians = [res.data.musician];
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

      const result = await api.post("/album", album);

      await saveTracks(a.key, result.data.album);
    }

    console.log("done saving albums");
  } catch (err) {
    console.error(err);
  }
}

async function saveMusicians() {
  try {
    const { data } = await plex.get("/library/sections/11/all");

    console.log(
      `Got ${data.MediaContainer.Metadata.length} musicians from plex.  About to start loop to save data into igloo.`
    );

    for (const artist of data.MediaContainer.Metadata) {
      const a = {
        name: artist.title,
        summary: artist.summary ? artist.summary : "",
      };

      if (artist.thumb) {
        a.thumb = await saveImage(
          artist.thumb,
          "public/images/musicians/thumb"
        );
      }

      if (artist.art) {
        a.art = await saveImage(artist.art, "public/images/musicians/art");
      }

      a.genres = await saveGenres(artist.Genre);

      await api.post("/musician", a);
    }

    console.log("done saving musicians");
  } catch (err) {
    console.error(err);
  }
}

async function saveGenres(genres) {
  if (!genres || !Array.isArray(genres) || genres.length === 0) {
    return [];
  }

  const genreList = [];

  for (const g of genres) {
    const res = await api.post("music-genre", { tag: g.tag });

    genreList.push(res.data.genre);
  }

  return genreList;
}

async function saveMoods(moods) {
  if (!moods || !Array.isArray(moods) || moods.length === 0) {
    return [];
  }

  const moodList = [];

  for (const m of moods) {
    const res = await api.post("/music-mood", { tag: m.tag });

    moodList.push(res.data.mood);
  }

  return moodList;
}
