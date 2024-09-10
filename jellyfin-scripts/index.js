import jellyfin from "./utils/jellyfin";
import api from "./utils/api";
import tmdb from "./utils/tmdb";

saveMovies();

async function saveMovies() {
  try {
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

    for (const i of data.Items) {
      const { data: tmdbMovie } = await tmdb.get(
        `/movie/${i.ProviderIds.Tmdb}`
      );

      const newMovie = {
        Title: tmdbMovie.title,
        FilePath: i.Path.replace("/data", "/plexmedia/zfs"),
        RunTime: i.MediaSources[0].RunTimeTicks,
        Tagline: tmdbMovie.tagline,
        Summary: tmdbMovie.overview,
        Budget: tmdbMovie.budget,
        Revenue: tmdbMovie.revenue,
        TmdbID: i.ProviderIds.Tmdb,
        ImdbID: i.ProviderIds.Imdb,
        ContentRating: i.OfficialRating,
        Year: i.ProductionYear,
        Genres: tmdbMovie.genres.map(g => ({
          Tag: g.name,
          GenreType: "movie",
        })),
      };

      if (tmdbMovie.poster_path) {
        newMovie.Thumb = `https://image.tmdb.org/t/p/original${tmdbMovie.poster_path}`;
      }

      if (tmdbMovie.backdrop_path) {
        newMovie.Art = `https://image.tmdb.org/t/p/original${tmdbMovie.backdrop_path}`;
      }

      if (i.CommunityRating) {
        newMovie.AudienceRating = i.CommunityRating;
      }

      if (i.RemoteTrailers) {
        newMovie.Trailers = i.RemoteTrailers.map(t => ({
          Title: t.Title,
          Url: t.Url,
        }));
      }

      if (tmdbMovie.release_date) {
        newMovie.ReleaseDate = new Date(tmdbMovie.release_date);
      }

      if (tmdbMovie.spoken_languages) {
        newMovie.SpokenLanguages = tmdbMovie.spoken_languages
          .map(l => l.english_name)
          .join(", ");
      }

      newMovie.Studios = tmdbMovie.production_companies.map(s => ({
        Name: s.name,
        Thumb: `https://image.tmdb.org/t/p/original${s.logo_path}`,
        Country: s.origin_country,
      }));

      const credits = await tmdb.get(`/movie/${newMovie.TmdbID}/credits`);

      newMovie.CastList = [];

      for (const c of credits.data.cast) {
        const res = await api.post("/artist", {
          Name: c.name,
          OriginalName: c.original_name,
          KnownFor: c.known_for_department,
          Thumb: `https://image.tmdb.org/t/p/original${c.profile_path}`,
        });

        newMovie.CastList.push({
          ArtistID: res.data.Artist.ID,
          Character: c.character,
          Order: c.order,
        });
      }

      newMovie.CrewList = [];

      for (const c of credits.data.crew) {
        const res = await api.post("/artist", {
          Name: c.name,
          OriginalName: c.original_name,
          KnownFor: c.known_for_department,
          Thumb: `https://image.tmdb.org/t/p/original${c.profile_path}`,
        });

        newMovie.CrewList.push({
          ArtistID: res.data.Artist.ID,
          Job: c.job,
          Department: c.department,
        });
      }

      await api.post("/movie", newMovie);
    }
  } catch (err) {
    console.error(err);
  }

  console.log("Finish saving movies");
}
