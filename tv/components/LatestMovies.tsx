import { ScrollView, View, Text, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import api from "@/lib/api";
import getError from "@/lib/getError";
import MovieCard from "./MovieCard";
import Loading from "./Loading";
import Alert from "./Alert";
import { light } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { MoviesResponse } from "@/types/Movie";

export default function LatestMovies() {
  const [showError, setShowError] = useState(true);

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/auth/movies/latest");

        if (!data.movies) {
          throw new Error("unable to get any movies");
        }

        return data.movies;
      } catch (err) {
        throw new Error(getError(err));
      }
    },
  });

  return (
    <View style={styles.container}>
      <Text style={styles.sectionTitle}>Latest Movies</Text>

      <View style={styles.contentContainer}>
        {isPending ? (
          <Loading message='Loading latest movies...' />
        ) : isError && showError ? (
          <Alert
            type='error'
            message={
              error instanceof Error ? error.message : "An error occurred"
            }
            onDismiss={() => setShowError(false)}
          />
        ) : (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.scrollViewContent}
            style={styles.scrollView}
          >
            {data?.map((movie, index) => (
              <View
                key={movie.ID}
                style={[
                  styles.movieCard,
                  index === data.length - 1 && styles.lastMovieCard,
                ]}
              >
                <MovieCard movie={movie} hasTVPreferredFocus={index === 0} />
              </View>
            ))}
          </ScrollView>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingVertical: Layout.spacing.xl,
  },
  sectionTitle: {
    fontSize: Layout.card.titleSize * 1.25,
    fontWeight: "bold",
    color: light,
    marginBottom: Layout.spacing.lg,
    paddingHorizontal: Layout.spacing.xl,
  },
  contentContainer: {
    height: Layout.card.height + Layout.spacing.xl * 2,
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
});
