import { FlatList, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { scale } from "react-native-size-matters";
import { memo, useCallback } from "react";
import MovieCard from "@/components/MovieCard";
import API_URL from "@/constants/Backend";
import type { SimpleMovie } from "@/types/Movie";
import type { PaginationResponse } from "@/types/Pagination";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";
import Spinner from "@/components/Spinner";

const MemoizedMovieCard = memo(({ movie, index }: { movie: SimpleMovie; index: number }) => (
  <ThemedView style={styles.movieCardContainer}>
    <MovieCard movie={movie} index={index} />
  </ThemedView>
));

MemoizedMovieCard.displayName = 'MemoizedMovieCard';

export default function MoviesScreen() {
  const { isPending, isError, data } = useQuery({
    queryKey: ["movies"],
    queryFn: async (): Promise<PaginationResponse<SimpleMovie>> => {
      try {
        const res = await fetch(`${API_URL}/movies`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error("a network error occurred while fetching movies");
      }
    },
  });

  // Memoized renderItem function
  const renderItem = useCallback(({ item, index }: { item: SimpleMovie; index: number }) => (
    <MemoizedMovieCard movie={item} index={index} />
  ), []);

  // Memoized keyExtractor
  const keyExtractor = useCallback((item: SimpleMovie) => item.id.toString(), []);

  if (isError) {
    return (
      <ThemedView style={styles.container}>
        <ThemedText type="subtitle" style={styles.errorText}>
          Error loading movies
        </ThemedText>
      </ThemedView>
    );
  }

  if (isPending) {
    return <Spinner />;
  }

  return (
    <ThemedView style={styles.container}>
      <ThemedView style={styles.header}>
        <ThemedText type="subtitle">All Movies</ThemedText>
      </ThemedView>

      <FlatList
        data={data?.items}
        numColumns={6}
        columnWrapperStyle={styles.row}
        showsVerticalScrollIndicator={false}
        keyExtractor={keyExtractor}
        renderItem={renderItem}
        contentContainerStyle={styles.listContent}
        removeClippedSubviews={true}
        maxToRenderPerBatch={10}
        windowSize={5}
        initialNumToRender={12}
        updateCellsBatchingPeriod={50}
      />
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  header: {
    paddingVertical: scale(10),
    paddingHorizontal: scale(20),
  },
  row: {
    justifyContent: "space-between",
    paddingHorizontal: scale(10),
    gap: scale(10),
  },
  movieCardContainer: {
    flex: 1,
    maxWidth: `${100 / 6}%`,
    marginBottom: scale(20),
  },
  listContent: {
    paddingVertical: scale(20),
  },
  errorText: {
    textAlign: "center",
    marginTop: scale(20),
  },
});
