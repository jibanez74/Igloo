// file handles grabing all movies from plex, assigning new data from tmdb and saves them to the Igloo data base

const fs = require("fs");
const path = require("path");
const api = require("./config/api");
const tmdb = require("./config/tmdb");
const savePlexImage = require("./utils/savePlexImage");
const saveTmdbImage = require("./utils/saveTmdbImage");
const plex = require("./config/plex");
const querystring = require("querystring");

saveMovies();

async function saveMovies() {
  try {
    const urlQueries =
      "?checkFiles=1&includeAllConcerts=1&includeBandwidths=1&includeChapters=1&includeChildren=1&includeConcerts=1&includeExtras=1&includeFields=1&includeGeolocation=1&includeLoudnessRamps=1&includeMarkers=1&includeOnDeck=1&includePopularLeaves=1&includePreferences=1&includeRelated=1&includeRelatedCount=1&includeReviews=1&includeStations=1";

    const res = await plex.get("/library/sections/1/all");

    for (const m of res.data.MediaContainer.Metadata) {
      const plexMovieData = await plex.get(m.key + urlQueries);

      const plexMovie = plexMovieData.data.MediaContainer.Metadata[0];

      const newMovie = {};

      if (plexMovie.Guid && plexMovie.Guid.length > 0) {
        plexMovie.Guid.forEach(g => {
          if (g.id.includes("tmdb")) {
            newMovie.tmdbID = g.id.split("://")[1];
          }

          if (g.id.includes("imdb")) {
            newMovie.imdbID = g.id.split("://")[1];
          }
        });
      }

      if (newMovie.tmdbID) {
        const tmdbMovie = await tmdb.get(`/movie/${newMovie.tmdbID}`);

        await saveMovieCredits(newMovie.tmdbID);

        newMovie.title = tmdbMovie.data.title;
        newMovie.tagline = tmdbMovie.data.tagline;
        newMovie.summary = tmdbMovie.data.overview;
        newMovie.releaseDate = new Date(tmdbMovie.data.release_date);
        newMovie.budget = tmdbMovie.data.budget;
        newMovie.revenue = tmdbMovie.data.revenue;
        newMovie.adult = tmdbMovie.data.adult;

        newMovie.genres = await saveGenres(tmdbMovie.data.genres);

        newMovie.studios = await saveStudios(
          tmdbMovie.data.production_companies
        );

        newMovie.thumb = await saveTmdbImage(
          tmdbMovie.data.poster_path,
          "public/images/thumb"
        );

        newMovie.art = await saveTmdbImage(
          tmdbMovie.data.backdrop_path,
          "public/images/movies/art"
        );
      } else {
        newMovie.title = plexMovie.title;
        newMovie.tagline = plexMovie.tagline;
        newMovie.summary = plexMovie.summary;
        newMovie.releaseDate = new Date(plexMovie.originallyAvailableAt);
      }

      newMovie.year = plexMovie.year;
    }
  } catch (err) {
    console.error(err);
  }
}

async function saveGenres(genres) {
  const result = [];

  try {
    for (const g of genres) {
      const { data } = await api.post("/movie/genre", {
        tag: g.name,
      });

      result.push(data.item);
    }

    return result;
  } catch (err) {
    throw err;
  }
}

async function saveStudios(studios) {
  const results = [];

  try {
    for (const s of studios) {
      const { data } = await api.post("/movie/studio", {
        title: s.name,
        logo: s.logo_path,
        country: s.origin_country,
      });

      results.push(data.item);
    }

    return results;
  } catch (err) {
    throw err;
  }
}

async function saveSpokenLanguages(spokenLanguages) {
  const results = [];

  try {
    for (const lang of spokenLanguages) {
      const { data } = await api.post("/movie/language", {
        title: lang.name,
        shortTitle: lang.iso_639_1,
        englishTitle: lang.english_name,
      });

      results.push(data.item);
    }
  } catch (err) {
    throw err;
  }
}

async function saveMovieCredits(id) {
  try {
    const { data } = await tmdb.get(`/movie/${id}/credits`);

    // write the data to a json file
    fs.writeFileSync(
      path.join(__dirname, "movieCredits.json"),
      JSON.stringify(data, null, 2)
    );
  } catch (err) {
    throw err;
  }
}
