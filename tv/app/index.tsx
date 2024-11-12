import { View, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "@/components/ThemedView";
import { ThemedText } from "@/components/ThemedText";
import Spinner from "@/components/Spinner";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

export default function HomeScreen() {
  const { data, isLoading, error } = useQuery<SimpleMovie[]>({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<SimpleMovie[]> => {
      try {
        const res = await api.get("/auth/movies/latest");
        return res.data.movies;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movies");
      }
    },
  });

  if (isLoading) {
    return <Spinner text='Loading latest movies...' />;
  }

  if (error) {
    return (
      <ThemedView variant='dark' style={styles.centerContainer}>
        <ThemedText variant='light' size='large' style={styles.errorText}>
          Error loading movies
        </ThemedText>
      </ThemedView>
    );
  }

  return (
    <ThemedView variant='dark' style={styles.container}>
      {/* Header Section */}
      <View style={styles.headerContainer}>
        <ThemedText
          variant='light'
          size='xlarge'
          weight='bold'
          style={styles.title}
        >
          Home Screen
        </ThemedText>

        <ThemedText variant='info' size='large' style={styles.subtitle}>
          The latest media in your server
        </ThemedText>
      </View>

      {/* Movies Grid */}
      <View style={styles.gridContainer}>
        {data?.map(movie => (
          <View key={movie.ID} style={styles.cardWrapper}>
            <MovieCard movie={movie} />
          </View>
        ))}
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 40,
  },
  centerContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  headerContainer: {
    alignItems: "center",
    marginBottom: 50,
  },
  title: {
    marginBottom: 16,
  },
  subtitle: {
    opacity: 0.8,
  },
  gridContainer: {
    flexDirection: "row",
    flexWrap: "wrap",
    justifyContent: "center",
    gap: 20,
  },
  cardWrapper: {
    width: "15.5%", // 6 cards per row with gap
    minWidth: 160,
    marginBottom: 20,
  },
  errorText: {
    textAlign: "center",
  },
});
