import { Image, Pressable, View, Text, StyleSheet } from "react-native";
import { Link } from "expo-router";
import getImgSrc from "@/lib/getImgSrc";
import { primary, dark, light, info, secondary } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  hasTVPreferredFocus?: boolean;
};

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
        {({ focused }) => (
          <>
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
                <Text
                  style={styles.title}
                  numberOfLines={1}
                  ellipsizeMode='tail'
                >
                  {movie.title}
                </Text>
                <Text
                  style={styles.year}
                  numberOfLines={1}
                  ellipsizeMode='tail'
                >
                  {movie.year}
                </Text>
              </View>
            </View>

            {/* Full Title Overlay (shows on focus) */}
            {focused && (
              <View style={styles.fullTitleContainer}>
                <View style={styles.fullTitleOverlay} />
                <View style={styles.fullTitleContent}>
                  <Text style={styles.fullTitle}>{movie.title}</Text>
                  <Text style={styles.fullYear}>{movie.year}</Text>
                </View>
              </View>
            )}
          </>
        )}
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
  // Full title overlay styles
  fullTitleContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  fullTitleOverlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: `${dark}CC`,
  },
  fullTitleContent: {
    padding: Layout.spacing.lg,
    alignItems: "center",
  },
  fullTitle: {
    fontSize: Layout.card.titleSize,
    fontWeight: "bold",
    color: light,
    textAlign: "center",
    marginBottom: Layout.spacing.sm,
  },
  fullYear: {
    fontSize: Layout.card.textSize,
    color: info,
    textAlign: "center",
  },
});
