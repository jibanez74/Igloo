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

    console.log(
      `Got ${res.data.MediaContainer.Metadata.length} movies from plex.  About to start loop to save data into igloo.`
    );

    for (const m of res.data.MediaContainer.Metadata) {
      const plexMovieData = await plex.get(m.key + urlQueries);

      const plexMovie = plexMovieData.data.MediaContainer.Metadata[0];

      // basic data from plex
      const newMovie = {
        duration: plexMovie.duration,
        contentRating: plexMovie.contentRating,
        year: plexMovie.year,
        criticRating: plexMovie.rating,
        audienceRating: plexMovie.audienceRating,
      };

      // check if the movie has a tmdb or imdb id
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

      // check if we have a tmdb id, if so get movie info from tmdb db
      if (newMovie.tmdbID) {
        const tmdbMovie = await tmdb.get(`/movie/${newMovie.tmdbID}`);

        newMovie.title = tmdbMovie.data.title;
        newMovie.summary = tmdbMovie.data.overview;
        newMovie.tagline = tmdbMovie.data.tagline;
        newMovie.releaseDate = new Date(tmdbMovie.data.release_date);
        newMovie.budget = tmdbMovie.data.budget;
        newMovie.revenue = tmdbMovie.data.revenue;

        newMovie.genres = await saveGenres(tmdbMovie.data.genres);

        newMovie.studios = await saveStudios(
          tmdbMovie.data.production_companies
        );

        if (tmdbMovie.data.spoken_languages) {
          newMovie.spokenLanguages = tmdbMovie.data.spoken_languages
            .map(l => l.english_name)
            .join("");
        }

        if (tmdbMovie.data.poster_path) {
          newMovie.thumb = await saveTmdbImage(
            tmdbMovie.data.poster_path,
            "public/images/thumb"
          );
        }

        if (tmdbMovie.data.backdrop_path) {
          newMovie.art = await saveTmdbImage(
            tmdbMovie.data.backdrop_path,
            "public/images/movies/art"
          );
        }

        const { crew, cast } = await saveMovieCredits(newMovie.tmdbID);

        const movieResult = await api.post("/movie", {
          movie: newMovie,
          crew,
          cast,
        });
      } else {
        newMovie.title = plexMovie.title;
        newMovie.tagline = plexMovie.tagline;
        newMovie.summary = plexMovie.summary;
        newMovie.releaseDate = new Date(plexMovie.originallyAvailableAt);

        await api.post("/movie", newMovie);
      }
    }

    console.log("done saving movies");
  } catch (err) {
    console.error(err.message);
    console.error(JSON.stringify(err.response.data));
  }
}

async function saveGenres(genres) {
  if (!genres || !Array.isArray(genres) || genres.length === 0) {
    return [];
  }

  const result = [];

  for (const g of genres) {
    const res = await api.post("/movie-genre", {
      tag: g.name,
    });

    result.push(res.data.genre);
  }

  return result;
}

async function saveStudios(studios) {
  if (!studios || !Array.isArray(studios) || studios.length === 0) {
    return [];
  }

  const results = [];

  for (const s of studios) {
    const res = await api.post("/studio", {
      title: s.name,
      thumb: s.logo_path,
      country: s.origin_country,
    });

    results.push(res.data.studio);
  }

  return results;
}

async function saveMovieCredits(tmdbMovieId) {
  const res = await tmdb.get(`/movie/${tmdbMovieId}/credits`);
  const cast = [];
  const crew = [];
  const staff = [...res.data.cast, ...res.data.crew];

  for (const c of res.data.cast) {
    const res = await api.post("/artist", {
      KnownFor: c.known_for_department,
      name: c.name,
      originalName: c.original_name,
      thumb: c.profile_path,
    });

    const { artist } = res.data;

    const member = {
      KnownFor: artist.known_for_department,
      name: artist.name,
      originalName: artist.original_name,
      thumb: artist.profile_path,
      artist,
    };

    if (Object.keys(c).includes("character")) {
      member.character = c.character;
      member.order = c.order;

      cast.push(member);
    } else {
      member.job = c.job;
      member.department = c.department;

      crew.push(member);
    }
  }

  return {
    crew,
    cast,
  };
}
