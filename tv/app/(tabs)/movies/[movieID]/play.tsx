import { View, StyleSheet } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import getImgSrc from "@/lib/getImgSrc";
import VideoPlayer from "@/components/VideoPlayer";
import API_URL from "../../../../constants/Backend";
import type { MovieDirectPlayData } from "../../../../types/Movie";
import Spinner from "@/components/Spinner";
import { ThemedText } from "@/components/ThemedText";

export default function PlayMovieScreen() {
  const { movieID } = useLocalSearchParams<{ movieID: string }>();
  
  const {
    isPending,
    isError,
    error,
    data: movie,
  } = useQuery({
    queryKey: ["play-movie", movieID],
    queryFn: async (): Promise<MovieDirectPlayData> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieID}/direct-play`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movie;
      } catch (err) {
        console.error(err);
        throw new Error("a network error occurred while fetching the movie");
      }
    },
    refetchOnWindowFocus: false,
    staleTime: 24 * 60 * 60 * 1000, // 24 hours in milliseconds
    gcTime: 24 * 60 * 60 * 1000, // 24 hours in milliseconds
  });

  if (isPending) {
    return (
      <View style={styles.container}>
        <Spinner />
      </View>
    );
  }

  if (isError) {
    return (
      <View style={styles.container}>
        <ThemedText type="subtitle" style={styles.errorText}>
          Error: {error.message}
        </ThemedText>
      </View>
    );
  }

  if (!movie) {
    return (
      <View style={styles.container}>
        <ThemedText type="subtitle" style={styles.errorText}>
          No movie data available
        </ThemedText>
      </View>
    );
  }

  const videoUri = `${API_URL}/media/movies${movie.file_path}`;
  console.log("Attempting to play video from:", videoUri);

  return (
    <View style={styles.container}>
      <VideoPlayer
        thumb={getImgSrc(movie.thumb)}
        title={movie.title}
        uri={videoUri}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "black",
    justifyContent: "center",
    alignItems: "center",
  },
  errorText: {
    color: "white",
    textAlign: "center",
  },
});
