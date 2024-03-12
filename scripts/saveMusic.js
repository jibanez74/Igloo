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
        genres: album.genres,
        album,
      };

      track.musicians = [album.musician];

      if (t.parentYear) {
        track.year = t.parentYear;
      }

      if (t.Mood) {
        track.moods = t.Mood.map(m => ({
          tag: m.tag,
        }));
      } else {
        track.moods = [{ tag: "unknown" }];
      }

      await api.post("/track", track);
    }

    return true;
  } catch (err) {
    throw err.response.data;
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

        album.musician = res.data.musician;
      }

      if (a.Genre) {
        album.genres = a.Genre.map(g => ({
          tag: g.tag,
        }));
      } else {
        album.genres = [
          {
            tag: "unknown",
          },
        ];
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

      album.ID = result.data.album.ID;

      await saveTracks(a.key, album);
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
        genres: [],
        genres: [],
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

      if (artist.Genre) {
        a.genres = artist.Genre.map(g => ({ tag: g.tag }));
      }

      await api.post("/musician", a);
    }

    console.log("done saving musicians");
  } catch (err) {
    console.error(err.response.data);
  }
}
