
import { useRef, useState, useEffect } from "react";
import {
  Image,
  Pressable,
  View,
  Text,
  StyleSheet,
  Animated,
  Dimensions,
} from "react-native";
import { Link } from "expo-router";
import { FlashList } from "@shopify/flash-list";
import {
  primary,
  dark,
  light,
  info,
  secondary,
} from "@/constants/Colors";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const scaleFactor = SCREEN_WIDTH / 1920;
const POSTER_WIDTH = Math.round(200 * scaleFactor);
const POSTER_HEIGHT = Math.round(300 * scaleFactor);
const CARD_MARGIN = Math.round(24 * scaleFactor);
const CONTAINER_PADDING = Math.round(32 * scaleFactor);

type MovieCardProps = {
  movie: SimpleMovie;
  hasTvFocus?: boolean;
  onFocus?: () => void;
  onBlur?: () => void;
};

export function MovieCard({
  movie,
  hasTvFocus = false,
  onFocus,
  onBlur,
}: MovieCardProps) {
  const [isFocused, setIsFocused] = useState(false);
  const scaleValue = useRef(new Animated.Value(1)).current;

  useEffect(() => {
    Animated.spring(scaleValue, {
      toValue: hasTvFocus ? 1.1 : 1,
      useNativeDriver: true,
    }).start();
  }, [hasTvFocus]);

  const handleFocus = () => {
    onFocus?.();
    setIsFocused(true);
  };

  const handleBlur = () => {
    onBlur?.();
    setIsFocused(false);
  };

  return (
    <Link
      href={{
        pathname: "/(tabs)/movies/[movieID]",
        params: { movieID: movie.ID },
      }}
      asChild
    >
      <Pressable
        accessibilityLabel={`Movie: ${movie.title}, released in ${movie.year}`}
        focusable={true}
        hasTVPreferredFocus={hasTvFocus}
        onFocus={handleFocus}
        onBlur={handleBlur}
        style={({ pressed }) => [
          styles.pressable,
          pressed && { opacity: 0.9 },
        ]}
      >
        <Animated.View
          style={[
            styles.card,
            {
              transform: [{ scale: scaleValue }],
            },
          ]}
        >
          <View style={styles.posterContainer}>
            <Image
              source={{ uri: getImgSrc(movie.thumb) }}
              style={styles.poster}
              resizeMode="cover"
              defaultSource={require("@/assets/placeholder.png")}
            />
          </View>
          <View style={styles.content}>
            <Text style={styles.title} numberOfLines={1} allowFontScaling>
              {movie.title}
            </Text>
            <Text style={styles.year}>{movie.year}</Text>
          </View>
          {isFocused && <View style={styles.focusBorder} />}
        </Animated.View>
      </Pressable>
    </Link>
  );
}

type MoviesGridProps = {
  movies: SimpleMovie[];
};

export function MoviesGrid({ movies }: MoviesGridProps) {
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
        keyExtractor={(item, index) =>
          item?.ID?.toString() || `movie-${index}`
        }
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
