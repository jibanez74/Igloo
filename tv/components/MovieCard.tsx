import { useEffect } from "react";
import { View, Pressable, StyleSheet } from "react-native";
import { Link } from "expo-router";
import { Image } from "expo-image";
import { scale } from "react-native-size-matters";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";
import { ThemedText } from "./ThemedText";
import { useThemeColor } from "@/hooks/useThemeColor";
import Animated, {
  useAnimatedStyle,
  withTiming,
  withDelay,
  useSharedValue,
} from "react-native-reanimated";

type MovieCardProps = {
  movie: SimpleMovie;
  index: number;
};

export default function MovieCard({ movie, index = 0 }: MovieCardProps) {
  const thumb = getImgSrc(movie.thumb);
  const tintColor = useThemeColor({}, "tint");
  const opacity = useSharedValue(0);

  useEffect(() => {
    opacity.value = withDelay(
      index * 100, // Stagger the animations based on index
      withTiming(1, { duration: 500 })
    );
  }, [index, opacity]);

  const animatedStyle = useAnimatedStyle(() => {
    return {
      opacity: opacity.value,
      transform: [{ scale: opacity.value }],
    };
  });

  return (
    <Link
      href={{
        pathname: "/(tabs)/movies/[movieID]",
        params: { movieID: movie.id.toString() },
      }}
      asChild
    >
      <Pressable
        style={({ focused, pressed }) => [
          styles.cardContainer,
          focused && { ...styles.focusedCard, borderColor: tintColor },
          pressed && styles.pressedCard,
        ]}
      >
        {({ focused, pressed }) => (
          <Animated.View style={[styles.innerContainer, animatedStyle]}>
            <View style={styles.imageContainer}>
              <Image
                source={thumb}
                contentFit="cover"
                allowDownscaling={true}
                cachePolicy="memory-disk"
                style={styles.image}
              />
            </View>
            <View style={styles.textContainer}>
              <ThemedText
                type="defaultSemiBold"
                style={[styles.titleText, focused && { color: tintColor }]}
                numberOfLines={2}
              >
                {movie.title}
              </ThemedText>
              <ThemedText type="default" style={styles.yearText}>
                {movie.year}
              </ThemedText>
            </View>
          </Animated.View>
        )}
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  cardContainer: {
    margin: scale(10),
    borderWidth: scale(2),
    borderColor: "transparent",
    borderRadius: scale(8),
    overflow: "hidden",
    backgroundColor: "transparent",
  },
  focusedCard: {
    borderWidth: scale(3),
    transform: [{ scale: 1.05 }],
  },
  pressedCard: {
    opacity: 0.8,
  },
  innerContainer: {
    flexDirection: "column",
    width: scale(200),
  },
  imageContainer: {
    width: scale(200),
    height: scale(300),
    backgroundColor: "rgba(0,0,0,0.1)",
  },
  image: {
    width: "100%",
    height: "100%",
  },
  textContainer: {
    paddingVertical: scale(10),
    paddingHorizontal: scale(5),
    backgroundColor: "transparent",
  },
  titleText: {
    fontSize: scale(16),
    marginBottom: scale(4),
  },
  yearText: {
    fontSize: scale(14),
    opacity: 0.7,
  },
});
