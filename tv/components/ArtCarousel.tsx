import { useState } from "react";
import { View, StyleSheet, Image, Pressable, Dimensions } from "react-native";
import { useRouter } from "expo-router";
import Carousel from "react-native-reanimated-carousel";
import LinearGradient from "react-native-linear-gradient";
import Colors from "@/constants/Colors";
import ThemedView from "./ThemedView";
import ThemedText from "./ThemedText";
import type { SimpleMovie } from "@/types/Movie";

type ArtCarouselProps = {
  movies: SimpleMovie[];
};

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get("window");
const CAROUSEL_HEIGHT = SCREEN_HEIGHT * 0.7;

export default function ArtCarousel({ movies }: ArtCarouselProps) {
  const router = useRouter();
  const [focusedId, setFocusedId] = useState<number | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const [loadedImages, setLoadedImages] = useState<Record<number, boolean>>({});

  const handleImageLoad = (id: number) => {
    setLoadedImages(prev => ({
      ...prev,
      [id]: true,
    }));
  };

  const renderItem = ({
    item: movie,
    index,
  }: {
    item: SimpleMovie;
    index: number;
  }) => (
    <View style={styles.itemContainer}>
      {/* Placeholder while image loads */}
      {!loadedImages[movie.ID] && (
        <ThemedView variant='dark' style={styles.imagePlaceholder}>
          <ThemedText variant='info' size='medium'>
            Loading...
          </ThemedText>
        </ThemedView>
      )}

      {/* Only render image if art exists */}
      {movie.art && (
        <Image
          source={{ uri: movie.art }}
          style={styles.backdropImage}
          resizeMode='cover'
          onLoad={() => handleImageLoad(movie.ID)}
          onError={() =>
            console.warn(`Failed to load image for movie: ${movie.ID}`)
          }
        />
      )}

      <LinearGradient
        colors={["transparent", `${Colors.dark}E6`]}
        style={styles.gradient}
      >
        <View style={styles.contentContainer}>
          <ThemedText
            variant='light'
            size='xlarge'
            weight='bold'
            style={styles.title}
          >
            {movie.title}
          </ThemedText>

          <ThemedText variant='info' size='large' style={styles.year}>
            {movie.year}
          </ThemedText>

          <View style={styles.buttonContainer}>
            <Pressable
              style={[styles.button, styles.playButton]}
              focusable={true}
              hasTVPreferredFocus={index === activeIndex}
              onFocus={() => setFocusedId(movie.ID)}
              onBlur={() => setFocusedId(null)}
              onPress={() => router.push(`/movies/${movie.ID}/watch`)}
            >
              <ThemedText variant='dark' size='large' weight='bold'>
                ▶ Play
              </ThemedText>
            </Pressable>

            <Pressable
              style={[styles.button, styles.moreButton]}
              focusable={true}
              onFocus={() => setFocusedId(movie.ID)}
              onBlur={() => setFocusedId(null)}
              onPress={() => router.push(`/movies/${movie.ID}`)}
            >
              <ThemedText variant='light' size='large' weight='bold'>
                More Info
              </ThemedText>
            </Pressable>
          </View>
        </View>
      </LinearGradient>
    </View>
  );

  return (
    <View style={styles.container}>
      <Carousel
        loop
        width={SCREEN_WIDTH}
        height={CAROUSEL_HEIGHT}
        data={movies}
        renderItem={renderItem}
        onSnapToItem={setActiveIndex}
        autoPlay={!focusedId}
        autoPlayInterval={8000}
        enabled={!focusedId}
        mode='parallax'
        modeConfig={{
          parallaxScrollingScale: 0.9,
          parallaxScrollingOffset: 50,
        }}
        style={styles.carousel}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    height: CAROUSEL_HEIGHT,
    backgroundColor: Colors.dark,
  },
  carousel: {
    width: SCREEN_WIDTH,
    height: CAROUSEL_HEIGHT,
  },
  itemContainer: {
    flex: 1,
    position: "relative",
  },
  imagePlaceholder: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  backdropImage: {
    width: "100%",
    height: "100%",
  },
  gradient: {
    position: "absolute",
    bottom: 0,
    left: 0,
    right: 0,
    height: CAROUSEL_HEIGHT * 0.6,
    justifyContent: "flex-end",
  },
  contentContainer: {
    padding: 40,
    paddingBottom: 60,
  },
  title: {
    fontSize: 48,
    marginBottom: 16,
    width: "50%",
  },
  year: {
    marginBottom: 24,
  },
  buttonContainer: {
    flexDirection: "row",
    gap: 20,
  },
  button: {
    paddingVertical: 16,
    paddingHorizontal: 32,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    transform: [{ scale: 1 }],
  },
  playButton: {
    backgroundColor: Colors.secondary,
    minWidth: 200,
  },
  moreButton: {
    backgroundColor: Colors.primary,
    borderWidth: 2,
    borderColor: Colors.secondary,
    minWidth: 150,
  },
});
