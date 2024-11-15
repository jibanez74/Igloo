import { View, StyleSheet, ViewStyle } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { FlashList } from "@shopify/flash-list";
import Colors from "@/constants/Colors";
import ThemedView from "@/components/ThemedView";
import ThemedText from "@/components/ThemedText";
import MovieCard from "@/components/MovieCard";
import ArtCarousel from "@/components/ArtCarousel";
import Spinner from "@/components/Spinner";
import useScale from "@/hooks/useScale";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

const ITEMS_PER_ROW = 6;
const BASE_GAP = 20;
const BASE_PADDING = 40;
const BASE_ITEM_WIDTH = 160;

export default function HomeScreen() {
  const scale = useScale();

  const itemGap = BASE_GAP * scale;
  const padding = BASE_PADDING * scale;
  const itemWidth = BASE_ITEM_WIDTH * scale;

  const movieCardStyle: ViewStyle = {
    width: itemWidth,
    margin: 0,
  };

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
          throw new Error("no movies");
        }

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

  if (isError || !movies) {
    return (
      <ThemedView variant='dark' style={styles.centerContainer}>
        <ThemedText variant='light' size='large'>
          Error loading movies
        </ThemedText>
      </ThemedView>
    );
  }

  return (
    <ThemedView variant='dark' style={styles.container}>
      {/* Featured Carousel */}
      <ArtCarousel movies={movies} />

      {/* Content Sections */}
      <View style={[styles.content, { padding }]}>
        {/* Latest Movies Row */}
        <View style={styles.section}>
          <ThemedText
            variant='light'
            size='large'
            weight='bold'
            style={styles.sectionTitle}
          >
            Latest Movies
          </ThemedText>

          <FlashList<SimpleMovie>
            data={movies}
            horizontal
            estimatedItemSize={itemWidth}
            renderItem={({ item: movie, index }) => (
              <MovieCard
                movie={movie}
                style={movieCardStyle}
                hasTVPreferredFocus={index === 0}
              />
            )}
            ItemSeparatorComponent={() => <View style={{ width: itemGap }} />}
            showsHorizontalScrollIndicator={false}
            keyExtractor={item => item.ID.toString()}
            contentContainerStyle={{ paddingRight: padding }}
          />
        </View>

        {/* You could add more sections here like:
        - Continue Watching
        - Popular on Igloo
        - Trending Now
        etc. */}
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
  content: {
    flex: 1,
  },
  section: {
    marginTop: 20,
  },
  sectionTitle: {
    marginBottom: 16,
  },
});
