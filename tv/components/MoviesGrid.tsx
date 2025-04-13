import { useRef, useState, useEffect } from "react";
import { Pressable, Text, View, StyleSheet, Dimensions } from "react-native";
import { FlashList } from "@shopify/flash-list";
import MovieCard from "./MovieCard";
import { secondary, info, primary, dark, light } from "@/constants/Colors";
import type { SimpleMovie } from "@/types/Movie";

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const scaleFactor = SCREEN_WIDTH / 1920;
const POSTER_WIDTH = Math.round(200 * scaleFactor);
const POSTER_HEIGHT = Math.round(300 * scaleFactor);
const CARD_MARGIN = Math.round(24 * scaleFactor);
const CONTAINER_PADDING = Math.round(32 * scaleFactor);

type MoviesGridProps = {
  movies: SimpleMovie[];
};

export default function MoviesGrid({ movies }: MoviesGridProps) {
  const availableWidth = SCREEN_WIDTH - CONTAINER_PADDING * 2;
  const numColumns = Math.floor(availableWidth / (POSTER_WIDTH + CARD_MARGIN));
  const itemHeight = POSTER_HEIGHT + CARD_MARGIN * 2;

  const [focusedIndex, setFocusedIndex] = useState(0);
  const flashListRef = useRef<FlashList<SimpleMovie>>(null);

  useEffect(() => {
    if (focusedIndex >= movies.length) {
      setFocusedIndex(0);
    }
  }, [movies.length]);

  useEffect(() => {
    flashListRef.current?.scrollToIndex({
      index: focusedIndex,
      animated: true,
    });
  }, [focusedIndex]);

  const renderMovie = ({
    item,
    index,
  }: {
    item: SimpleMovie;
    index: number;
  }) => (
    <View
      style={[
        gridStyles.movieContainer,
        (index + 1) % numColumns === 0 && gridStyles.lastInRow,
      ]}
    >
      <MovieCard
        movie={item}
        hasTvFocus={focusedIndex === index}
        onFocus={() => setFocusedIndex(index)}
      />
    </View>
  );

  if (movies.length === 0) {
    return (
      <View style={gridStyles.emptyContainer}>
        <Text style={gridStyles.emptyText}>No movies available</Text>
      </View>
    );
  }

  return (
    <View style={gridStyles.container}>
      <FlashList
        ref={flashListRef}
        data={movies}
        renderItem={renderMovie}
        numColumns={numColumns}
        estimatedItemSize={itemHeight}
        contentContainerStyle={gridStyles.contentContainer}
        showsVerticalScrollIndicator={false}
        keyExtractor={(item, index) => item?.id?.toString() || `movie-${index}`}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  pressable: {
    margin: 8,
  },
  card: {
    width: POSTER_WIDTH,
    backgroundColor: `${primary}33`,
    borderRadius: 8,
    overflow: "hidden",
  },
  posterContainer: {
    width: POSTER_WIDTH,
    height: POSTER_HEIGHT,
    backgroundColor: dark,
  },
  poster: {
    width: "100%",
    height: "100%",
  },
  content: {
    padding: 12,
  },
  title: {
    fontSize: 16,
    fontWeight: "bold",
    color: light,
    marginBottom: 4,
  },
  year: {
    fontSize: 14,
    color: info,
  },
  focusBorder: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    borderWidth: 3,
    borderColor: secondary,
    borderRadius: 8,
  },
});

const gridStyles = StyleSheet.create({
  container: {
    flex: 1,
    width: "100%",
  },
  contentContainer: {
    padding: CONTAINER_PADDING,
  },
  movieContainer: {
    marginRight: CARD_MARGIN,
    marginBottom: CARD_MARGIN,
  },
  lastInRow: {
    marginRight: 0,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  emptyText: {
    fontSize: 18,
    color: "gray",
  },
});
