import { ScrollView, View, Text, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import api from "@/lib/api";
import MovieCard from "./MovieCard";
import { light, info, danger } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { MoviesResponse } from "@/types/Movie";

export default function LatestMovies() {
  const { data, isPending, isError } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/auth/movies/latest");

        if (!data.movies) {
          throw new Error("unable to get any movies");
        }

        return data.movies;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movies from server");
      }
    },
  });

  if (isPending) {
    return (
      <View style={styles.container}>
        <Text style={styles.loadingText}>Loading latest movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View style={styles.container}>
        <Text style={styles.errorText}>Unable to get movies from server</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.sectionTitle}>Latest Movies</Text>

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={styles.scrollViewContent}
        style={styles.scrollView}
      >
        {data.map((movie, index) => (
          <View
            key={movie.ID}
            style={[
              styles.movieCard,
              index === data.length - 1 && styles.lastMovieCard,
            ]}
          >
            <MovieCard movie={movie} hasTVPreferredFocus={false} />
          </View>
        ))}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingVertical: Layout.spacing.xl,
  },
  sectionTitle: {
    fontSize: Layout.card.titleSize * 1.25, // 25% larger than card title
    fontWeight: "bold",
    color: light,
    marginBottom: Layout.spacing.lg,
    paddingHorizontal: Layout.spacing.xl,
  },
  scrollView: {
    flexGrow: 0,
  },
  scrollViewContent: {
    paddingHorizontal: Layout.spacing.xl,
  },
  movieCard: {
    marginRight: Layout.card.spacing,
  },
  lastMovieCard: {
    marginRight: 0,
  },
  loadingText: {
    fontSize: Layout.card.textSize,
    color: info,
    paddingHorizontal: Layout.spacing.xl,
  },
  errorText: {
    fontSize: Layout.card.textSize,
    color: danger,
    paddingHorizontal: Layout.spacing.xl,
  },
});
