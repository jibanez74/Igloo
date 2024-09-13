import asyncHandler from "../lib/asyncHandler";
import jellyfin from "../lib/jellyfin";
import tmdb from "../lib/tmdb";
import Movie from "../models/Movie";
import Genre from "../models/Genre";
import Studio from "../models/Studio";
import Artist from "../models/Artist";

// todo
// this controller needs to be updated to process multiple badges
export const saveJellyfinMovies = asyncHandler(async (req, res, next) => {
  const batchSize = 400;
  let processedCount = 0;

  const genres = await Genre.find();
  const studios = await Studio.find();

  const genreMap = new Map(genres.map(g => [g.tag, g]));
  const studioMap = new Map(studios.map(s => [s.name, s]));

  const { data } = await jellyfin.get(
    `/Users/${process.env.JELLYFIN_USERID}/Items`,
    {
      params: {
        Recursive: true,
        IncludeItemTypes: "Movie",
        Fields:
          "Chapters, MediaSources, Path, ProviderIds, RemoteTrailers, MediaStreams, Width, Height",
        hasTmdbId: true,
      },
    }
  );

  console.log(`Got ${data.Items.length} movies with tmdb id`);

  for (let i = 0; i < data.Items.length; i += batchSize) {
    const batch = data.Items.slice(i, i + batchSize);
    const moviePromises = batch.map(async item => {
      const { data: tmdbMovie } = await tmdb.get(
        `/movie/${item.ProviderIds.Tmdb}`
      );

      const exist = await Movie.findOne({ Title: tmdbMovie.title });

      if (!exist) {
        const newMovie = new Movie({
          title: tmdbMovie.title,
          filePath: item.Path.replace("/data", "/plexmedia/zfs"),
          runTime: item.MediaSources[0].RunTimeTicks,
          tagline: tmdbMovie.tagline,
          summary: tmdbMovie.overview,
          budget: tmdbMovie.budget,
          revenue: tmdbMovie.revenue,
          tmdbID: item.ProviderIds.Tmdb,
          imdbID: item.ProviderIds.Imdb,
          contentRating: item.OfficialRating,
          year: item.ProductionYear,
          thumb: tmdbMovie.poster_path
            ? `https://image.tmdb.org/t/p/original${tmdbMovie.poster_path}`
            : undefined,
          art: tmdbMovie.backdrop_path
            ? `https://image.tmdb.org/t/p/original${tmdbMovie.backdrop_path}`
            : undefined,
          audienceRating: item.CommunityRating,
          trailers: item.RemoteTrailers
            ? item.RemoteTrailers.map(t => ({ title: t.Title, url: t.Url }))
            : [],
          releaseDate: tmdbMovie.release_date
            ? new Date(tmdbMovie.release_date)
            : undefined,
          spokenLanguages: tmdbMovie.spoken_languages
            ? tmdbMovie.spoken_languages.map(l => l.english_name).join(", ")
            : undefined,
        });

        newMovie.genres = tmdbMovie.genres.map(g => {
          let genre = genreMap.get(g.name);
          if (!genre) {
            genre = new Genre({ tag: g.name, genreType: "movie" });
            genreMap.set(g.name, genre);
          }
          return genre._id;
        });

        newMovie.studios = tmdbMovie.production_companies.map(s => {
          let studio = studioMap.get(s.name);
          if (!studio) {
            studio = new Studio({
              name: s.name,
              country: s.origin_country,
              thumb: `https://image.tmdb.org/t/p/original${s.logo_path}`,
            });
            studioMap.set(s.name, studio);
          }

          return studio._id;
        });

        const { data: credits } = await tmdb.get(
          `/movie/${newMovie.tmdbID}/credits`
        );

        newMovie.castList = await Promise.all(
          credits.cast.map(async c => {
            let artist = await Artist.findOne({ name: c.name });
            if (!artist) {
              artist = new Artist({
                name: c.name,
                originalName: c.original_name,
                knownFor: c.known_for_department,
                thumb: c.profile_path
                  ? `https://image.tmdb.org/t/p/original${c.profile_path}`
                  : undefined,
              });

              await artist.save();
            }

            return {
              artist: artist._id,
              character: c.character,
              order: c.order,
            };
          })
        );

        newMovie.crewList = await Promise.all(
          credits.crew.map(async c => {
            let artist = await Artist.findOne({ name: c.name });
            if (!artist) {
              artist = new Artist({
                name: c.name,
                originalName: c.original_name,
                knownFor: c.known_for_department,
                thumb: c.profile_path
                  ? `https://image.tmdb.org/t/p/original${c.profile_path}`
                  : undefined,
              });

              await artist.save();
            }

            return { artist: artist._id, job: c.job, department: c.department };
          })
        );

        return newMovie;
      }
    });

    const newMovies = (await Promise.all(moviePromises)).filter(Boolean);

    if (newMovies.length > 0) {
      await Movie.insertMany(newMovies);
      processedCount += newMovies.length;
      console.log(`Processed ${processedCount} new movies`);
    }
  }

  res.status(201).json({
    message: `${processedCount} Jellyfin movies were transferred successfully`,
  });
});
