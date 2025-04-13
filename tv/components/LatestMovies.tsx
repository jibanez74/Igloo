import { useState } from "react";
import { FlatList, View, Text, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import getError from "@/lib/getError";
import api from "@/lib/api";
import { dark, light } from "@/constants/Colors";
import Loading from "./Loading";
import Alert from "./Alert";
import MovieCard from "./MovieCard";
import type { SimpleMovie } from "@/types/Movie";

// Card dimensions based on MovieCard
const CARD_WIDTH = 200;
const CARD_HEIGHT = 300;
const CONTAINER_PADDING = 32;
const CARD_MARGIN = 24;

type MoviesResponse = {
  movies: SimpleMovie[];
};

export default function LatestMovies() {
  const [showError, setShowError] = useState(true);

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<MoviesResponse> => {
      try {
        const { data } = await api.get("/movies/latest");

        return data
      } catch (err) {
        throw new Error(getError(err));
      }
    },
  });

  const renderMovie = ({
    item,
    index,
  }: {
    item: SimpleMovie;
    index: number;
  }) => (
    <View
      style={[
        styles.movieContainer,
        index === (data?.movies.length || 0) - 1 && styles.lastMovie,
      ]}
    >
      <MovieCard movie={item} hasTvFocus={index === 0} />
    </View>
  );

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
          <FlatList
            data={data?.movies}
            renderItem={renderMovie}
            keyExtractor={item => item.id.toString()}
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.listContent}
          />
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingVertical: CONTAINER_PADDING,
    backgroundColor: dark,
  },
  sectionTitle: {
    fontSize: 28,
    fontWeight: "bold",
    color: light,
    marginBottom: 24,
    paddingHorizontal: CONTAINER_PADDING,
  },
  contentContainer: {
    height: CARD_HEIGHT + CONTAINER_PADDING, // Based on MovieCard height + padding
  },
  listContent: {
    paddingHorizontal: CONTAINER_PADDING,
  },
  movieContainer: {
    marginRight: CARD_MARGIN,
  },
  lastMovie: {
    marginRight: 0,
  },
});
