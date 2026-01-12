import type {
  AlbumDetailsResponseType,
  ApiFailureType,
  ApiResponseType,
  MovieDetailsType,
  MusicStatsType,
  ShuffleTracksResponseType,
  SimpleAlbumType,
  TheaterMovieType,
  TracksListResponseType,
} from "@/types";

const ERROR_NOTFOUND: ApiFailureType = {
  error: true,
  message: "404 - The resource you requested was not found.",
};

const NETWORK_ERROR: ApiFailureType = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
};

export async function login(email: string, password: string) {
  try {
    const res = await fetch("/api/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include",
      body: JSON.stringify({ email, password }),
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    return res.json();
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getAuthUser() {
  try {
    const res = await fetch("/api/auth/user", {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    return res.json();
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

// functions for home page
export async function getLatestAlbums(): Promise<
  ApiResponseType<{ albums: SimpleAlbumType[] }>
> {
  try {
    const res = await fetch("/api/music/albums/latest", {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<{ albums: SimpleAlbumType[] }> =
      await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getMoviesInTheaters(): Promise<
  ApiResponseType<{ movies: TheaterMovieType[] }>
> {
  try {
    const res = await fetch("/api/tmdb/movies/in-theaters", {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<{ movies: TheaterMovieType[] }> =
      await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getMovieInTheaterDetails(
  id: number
): Promise<ApiResponseType<{ movie: MovieDetailsType }>> {
  try {
    const res = await fetch(`/api/tmdb/movies/${id}`, {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<{ movie: MovieDetailsType }> = await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getAlbumDetails(
  id: number
): Promise<ApiResponseType<AlbumDetailsResponseType>> {
  try {
    const res = await fetch(`/api/music/albums/details/${id}`, {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<AlbumDetailsResponseType> = await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getTracksPaginated(
  limit: number,
  offset: number
): Promise<ApiResponseType<TracksListResponseType>> {
  try {
    const res = await fetch(`/api/music/tracks?limit=${limit}&offset=${offset}`, {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<TracksListResponseType> = await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getMusicStats(): Promise<ApiResponseType<MusicStatsType>> {
  try {
    const res = await fetch("/api/music/stats", {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<MusicStatsType> = await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getShuffleTracks(
  limit: number = 50
): Promise<ApiResponseType<ShuffleTracksResponseType>> {
  try {
    const res = await fetch(`/api/music/tracks/shuffle?limit=${limit}`, {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<ShuffleTracksResponseType> = await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}
