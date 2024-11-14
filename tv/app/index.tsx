import { useState } from "react";
import { View, StyleSheet, Dimensions } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { FlashList } from "@shopify/flash-list";
import Colors from "@/constants/Colors";
import ThemedView from "@/components/ThemedView";
import ThemedText from "@/components/ThemedText";
import MovieCard from "@/components/MovieCard";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const COLUMN_COUNT = 6;
const ITEM_GAP = 20;
const ITEM_WIDTH =
  (SCREEN_WIDTH - 40 * 2 - ITEM_GAP * (COLUMN_COUNT - 1)) / COLUMN_COUNT;

export default function HomeScreen() {
  const [showError, setShowError] = useState(false);

  const {
    data: movies,
    isLoading,
    error,
    isError,
  } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const res = await api.get("/auth/movies/latest");

        if (!res.data.movies) {
          throw new Error("no movies found");
        }

        res.data.movies;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch latest movies");
      }
    },
  });

  if (isLoading) {
    return <Spinner text='Loading latest movies...' />;
  }

  if (isError) {
    return (
      <Alert
        visible={true}
        title='Error'
        message={error?.message || "Failed to load movie"}
        onConfirm={() => setShowError(false)}
        onCancel={() => setShowError(false)}
      />
    );
  }

  return (
    <ThemedView variant='dark' style={styles.container}>
      {/* Header Section */}
      <View style={styles.header}>
        <ThemedText
          variant='light'
          size='xlarge'
          weight='bold'
          style={styles.title}
        >
          Welcome to Igloo
        </ThemedText>
        <ThemedText variant='info' size='large' style={styles.subtitle}>
          The latest media in your server
        </ThemedText>
      </View>

      {/* Movies Grid */}
      <View style={styles.gridContainer}>
        <FlashList
          data={movies}
          numColumns={COLUMN_COUNT}
          estimatedItemSize={ITEM_WIDTH}
          renderItem={({ item: movie, index }) => (
            <MovieCard
              movie={movie}
              style={styles.movieCard}
              hasTVPreferredFocus={index === 0}
            />
          )}
          ItemSeparatorComponent={() => <View style={{ width: ITEM_GAP }} />}
          contentContainerStyle={styles.gridContent}
          showsVerticalScrollIndicator={false}
          keyExtractor={item => item.ID.toString()}
        />
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  centerContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  header: {
    padding: 40,
    paddingBottom: 20,
  },
  title: {
    marginBottom: 8,
  },
  subtitle: {
    opacity: 0.8,
  },
  gridContainer: {
    flex: 1,
    paddingHorizontal: 40,
  },
  gridContent: {
    gap: ITEM_GAP,
    padding: 4,
  },
  movieCard: {
    width: ITEM_WIDTH,
    margin: 0,
  },
});
