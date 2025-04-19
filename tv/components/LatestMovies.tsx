import { TVFocusGuideView, View, FlatList, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { scale } from "react-native-size-matters";
import MovieCard from "@/components/MovieCard";
import API_URL from "@/constants/Backend";
import type { MoviesResponse } from "@/types/Movie";
import { ThemedText } from "./ThemedText";
import Spinner from "./Spinner";

type MoviePlaybackResponse = {
  id: number;
  title: string;
  thumb: string;
  file_path: string;
};

export function useMoviePlayback(movieId: number) {
  return useQuery<MoviePlaybackResponse>({
    queryKey: ["movie-playback", movieId],
    queryFn: async (): Promise<MoviePlaybackResponse> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieId}/playback`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error(
          "a network error occurred while fetching movie playback"
        );
      }
    },
    enabled: !!movieId,
  });
}

export default function LatestMovies() {
  const { isPending, isError, error, data } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<MoviesResponse> => {
      try {
        const res = await fetch(`${API_URL}/movies/latest`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error(
          "a network error occurred while fetching latest movies"
        );
      }
    },
  });

  if (isError) {
    console.error(error);
  }

  if (isPending) {
    return <Spinner />;
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <ThemedText type="subtitle">
          Latest Movies
        </ThemedText>
      </View>

      <TVFocusGuideView
        style={styles.focusGuide}
        destinations={[]}
        trapFocusLeft={true}
        trapFocusRight={true}
        autoFocus={true}
      >
        <FlatList
          data={data?.movies}
          horizontal
          showsHorizontalScrollIndicator={false}
          keyExtractor={(item) => item.id.toString()}
          renderItem={({ item, index }) => (
            <View style={styles.movieCardContainer}>
              <MovieCard movie={item} index={index} />
            </View>
          )}
          contentContainerStyle={styles.listContent}
        />
      </TVFocusGuideView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  focusGuide: {
    flex: 1,
  },
  header: {
    paddingVertical: scale(10),
    paddingHorizontal: scale(20),
  },
  movieCardContainer: {
    marginRight: scale(20),
  },
  listContent: {
    paddingHorizontal: scale(20),
  },
});
