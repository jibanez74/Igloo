import { Image, Pressable, View, Text, StyleSheet } from "react-native";
import { Link } from "expo-router";
import getImgSrc from "@/lib/getImgSrc";
import { primary, dark, light, info, secondary } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { MovieCardProps } from "@/types/Movie";

export default function MovieCard({
  movie,
  hasTVPreferredFocus = false,
}: MovieCardProps) {
  return (
    <Link
      asChild
      href={{
        pathname: "/(tabs)/movies/[movieID]",
        params: { movieID: movie.ID },
      }}
    >
      <Pressable
        focusable={true}
        hasTVPreferredFocus={hasTVPreferredFocus}
        style={({ focused }) => [styles.card, focused && styles.cardFocused]}
      >
        {/* Thumbnail */}
        <View style={styles.imageContainer}>
          <Image
            source={{ uri: getImgSrc(movie.thumb) }}
            style={styles.image}
            resizeMode='cover'
          />

          {/* Gradient Overlay */}
          <View style={styles.overlay} />
        </View>

        {/* Content */}
        <View style={styles.content}>
          <View style={styles.textContainer}>
            <Text style={styles.title} numberOfLines={1} ellipsizeMode='tail'>
              {movie.title}
            </Text>
            <Text style={styles.year} numberOfLines={1} ellipsizeMode='tail'>
              {movie.year}
            </Text>
          </View>
        </View>
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  card: {
    width: Layout.card.width,
    borderRadius: Layout.spacing.sm,
    overflow: "hidden",
    backgroundColor: `${primary}33`,
    transform: [{ scale: 1 }],
    borderWidth: 2,
    borderColor: "transparent",
  },
  cardFocused: {
    transform: [{ scale: 1.05 }],
    backgroundColor: `${primary}66`,
    borderColor: secondary,
  },
  imageContainer: {
    position: "relative",
    width: Layout.card.width,
    height: Layout.card.height,
  },
  image: {
    width: "100%",
    height: "100%",
  },
  overlay: {
    position: "absolute",
    left: 0,
    right: 0,
    top: 0,
    bottom: 0,
    backgroundColor: `${dark}99`,
  },
  content: {
    padding: Layout.spacing.md,
    width: Layout.card.width,
  },
  textContainer: {
    width: "100%",
  },
  title: {
    fontSize: Layout.card.titleSize,
    fontWeight: "bold",
    color: light,
    marginBottom: Layout.spacing.sm,
    width: "100%",
  },
  year: {
    fontSize: Layout.card.textSize,
    color: info,
    width: "100%",
  },
});
