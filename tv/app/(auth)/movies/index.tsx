import { View, Text, StyleSheet } from "react-native";
import { FlashList } from "@shopify/flash-list";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

type MoviesResponse = {
  movies: SimpleMovie[];
  count: number;
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#121F32",
  },
  header: {
    paddingHorizontal: 32,
    paddingVertical: 24,
  },
  headerText: {
    color: "#F3F0E8",
    fontSize: 36,
    fontWeight: "bold",
  },
  gridContainer: {
    flex: 1,
  },
  movieItem: {
    width: "16.666%",
    paddingHorizontal: 16,
    paddingVertical: 16,
  },
  contentContainer: {
    paddingBottom: 32,
  },
  // Skeleton styles
  skeletonCard: {
    width: 200,
    height: 300,
    backgroundColor: "rgba(28, 57, 94, 0.3)", // primary/30
    borderRadius: 8,
    overflow: "hidden",
  },
  skeletonTitle: {
    width: "80%",
    height: 24,
    backgroundColor: "rgba(28, 57, 94, 0.3)",
    marginTop: 16,
    borderRadius: 4,
  },
  skeletonYear: {
    width: "40%",
    height: 20,
    backgroundColor: "rgba(28, 57, 94, 0.3)",
    marginTop: 8,
    borderRadius: 4,
  },
});

export default function MoviesScreen() {
  const { data, error, isError, isPending } = useQuery({
    queryKey: ["movies"],
    queryFn: async () => {
      const { data } = await api.get<MoviesResponse>("/auth/movies/all");
      return data;
    },
    staleTime: 5 * 60 * 1000,
  });

  // Loading skeleton data
  const skeletonData = Array(24).fill(null); // 4 rows of 6 items

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerText}>
          All Movies ({!isPending ? data?.count || 0 : "..."})
        </Text>
      </View>

      <View style={styles.gridContainer}>
        <FlashList
          data={isPending ? skeletonData : data?.movies}
          numColumns={6}
          estimatedItemSize={375}
          keyExtractor={(item, index) =>
            item?.ID?.toString() || `skeleton-${index}`
          }
          renderItem={({ item, index }) => (
            <View style={styles.movieItem}>
              {isPending ? (
                // Skeleton loader
                <View>
                  <View style={styles.skeletonCard} />
                  <View style={styles.skeletonTitle} />
                  <View style={styles.skeletonYear} />
                </View>
              ) : (
                <MovieCard movie={item} hasTVPreferredFocus={index === 0} />
              )}
            </View>
          )}
          contentContainerStyle={styles.contentContainer}
        />
      </View>

      {isError && (
        <View
          style={[
            styles.container,
            {
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              justifyContent: "center",
              alignItems: "center",
            },
          ]}
        >
          <Text className='text-danger text-2xl'>
            {error?.message || "Error loading movies"}
          </Text>
        </View>
      )}
    </View>
  );
}
