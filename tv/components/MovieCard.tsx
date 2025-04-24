import { useEffect } from "react";
import { View, Pressable, StyleSheet } from "react-native";
import { useRouter } from "expo-router";
import { Image } from "expo-image";
import { Colors } from "@/constants/Colors";
import { useColorScheme } from "react-native";
import { ThemedText } from "./ThemedText";
import Animated, {
  useAnimatedStyle,
  withTiming,
  withDelay,
  useSharedValue,
  withSequence,
} from "react-native-reanimated";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  index: number;
};

export default function MovieCard({ movie, index }: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);
  const router = useRouter();
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme ?? "light"];
  const opacity = useSharedValue(0);
  const scale = useSharedValue(0.9);

  useEffect(() => {
    opacity.value = withDelay(
      index * 100, // Stagger the animation based on index
      withSequence(
        withTiming(1, { duration: 300 }),
        withTiming(1, { duration: 100 })
      )
    );
    scale.value = withDelay(index * 100, withTiming(1, { duration: 300 }));
  }, []);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
    transform: [{ scale: scale.value }],
  }));

  return (
    <Pressable
      onPress={() =>
        router.navigate({
          pathname: "/(tabs)/movies/[movieID]",
          params: {
            movieID: movie.id.toString(),
          },
        })
      }
      hasTVPreferredFocus={index === 0}
      style={styles.container}
    >
      <Animated.View
        style={[
          styles.card,
          { backgroundColor: colors.background },
          animatedStyle,
        ]}
      >
        <View style={styles.imageContainer}>
          <Image
            source={imgSrc}
            contentFit="cover"
            cachePolicy="memory-disk"
            allowDownscaling
            style={styles.image}
          />
        </View>

        <View style={styles.textContainer}>
          <ThemedText
            type="defaultSemiBold"
            style={styles.title}
            numberOfLines={1}
          >
            {movie.title}
          </ThemedText>
          <ThemedText type="subtitle" style={styles.year}>
            {movie.year}
          </ThemedText>
        </View>
      </Animated.View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: {
    margin: 8,
    width: 200,
  },
  card: {
    borderRadius: 8,
    overflow: "hidden",
    elevation: 3,
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 3.84,
  },
  imageContainer: {
    width: "100%",
    aspectRatio: 2 / 3,
  },
  image: {
    width: "100%",
    height: "100%",
  },
  textContainer: {
    padding: 8,
  },
  title: {
    marginBottom: 4,
  },
  year: {
    opacity: 0.7,
  },
});
