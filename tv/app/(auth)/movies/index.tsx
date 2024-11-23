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
    backgroundColor: '#121F32', // matches bg-dark
  },
  header: {
    paddingHorizontal: 32,
    paddingVertical: 24,
  },
  headerText: {
    color: '#F3F0E8', // matches text-light
    fontSize: 36,
    fontWeight: 'bold',
  },
  gridContainer: {
    flex: 1,
  },
  movieItem: {
    width: '16.666%',
    paddingHorizontal: 16,
    paddingVertical: 16,
  },
  contentContainer: {
    paddingBottom: 32,
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

  if (isPending) {
    return (
      <View style={styles.container}>
        <Text className="text-info text-2xl p-8">Loading movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View style={styles.container}>
        <Text className="text-danger text-2xl p-8">
          {error?.message || "Error loading movies"}
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerText}>
          All Movies ({data?.count || 0})
        </Text>
      </View>

      <View style={styles.gridContainer}>
        <FlashList
          data={data?.movies}
          numColumns={6}
          estimatedItemSize={375}
          keyExtractor={item => item.ID.toString()}
          renderItem={({ item, index }) => (
            <View style={styles.movieItem}>
              <MovieCard 
                movie={item} 
                hasTVPreferredFocus={index === 0} 
              />
            </View>
          )}
          contentContainerStyle={styles.contentContainer}
        />
      </View>
    </View>
  );
}
