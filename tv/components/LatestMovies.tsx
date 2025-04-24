import { FlatList, View, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import API_URL from "@/constants/Backend";
import MovieCard from "./MovieCard";
import type { SimpleMovie } from "@/types/Movie";
import { ThemedText } from "./ThemedText";

export default function LatestMovies() {
  const { isPending, isError, error, data } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<SimpleMovie[]> => {
      try {
        const res = await fetch(`${API_URL}/movies/latest`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movies;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch latest movies");
      }
    },
  });

  if (isPending) {
    return (
      <View style={styles.container}>
        <ThemedText type="title" style={styles.title}>Loading...</ThemedText>
      </View>
    );
  }

  if (isError) {
    return (
      <View style={styles.container}>
        <ThemedText type="title" style={styles.title}>Error: {error.message}</ThemedText>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <ThemedText type="title" style={styles.title}>Latest Movies</ThemedText>
      <FlatList
        data={data}
        renderItem={({ item, index }) => (
          <MovieCard movie={item} index={index} />
        )}
        keyExtractor={(item) => item.id.toString()}
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={styles.listContent}
        initialNumToRender={5}
        maxToRenderPerBatch={5}
        windowSize={5}
        getItemLayout={(data, index) => ({
          length: 216, // 200 (card width) + 16 (margin)
          offset: 216 * index,
          index,
        })}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginVertical: 24,
  },
  title: {
    marginBottom: 16,
    marginLeft: 16,
  },
  listContent: {
    paddingHorizontal: 8,
  },
});
