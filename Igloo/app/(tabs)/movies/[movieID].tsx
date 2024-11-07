import {
  StyleSheet,
  View,
  Text,
  Pressable,
  Image,
  ScrollView,
  SafeAreaView,
} from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import { handleApiError } from "@/lib/getError";
import api from "@/lib/api";
import Colors from "@/constants/Colors";
import type { Movie } from "@/types/Movie";
import { useState } from "react";
import ExoVideoPlayer from "@/components/ExoVideoPlayer";

type MovieResponse = {
  movie: Movie;
};

const testVideoAsset = require("@/assets/videos/test.mp4");

export default function MovieDetailScreen() {
  const { movieID } = useLocalSearchParams<{ movieID: string }>();
  const [isPlaying, setIsPlaying] = useState(false);

  const {
    data: movie,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: () =>
      handleApiError(async () => {
        const response = await api.get<MovieResponse>(
          `/auth/movies/${movieID}`
        );
        return response.data.movie;
      }),
  });

  if (isLoading) {
    return <Spinner message='Loading movie details...' />;
  }

  if (isError || !movie) {
    return (
      <Alert
        title='Error Loading Movie'
        msg={error?.message}
        variant='danger'
      />
    );
  }

  if (isPlaying) {
    return (
      <ExoVideoPlayer
        uri={movie.filePath}
        audioTracks={movie.audioList}
        subtitleTracks={movie.subtitleList}
      />
    );
  }

  return (
    <SafeAreaView style={styles.safeArea}>
      <ScrollView style={styles.container}>
        {/* Background Art Image */}
        <Image
          source={{ uri: movie.art }}
          style={styles.backgroundArt}
          resizeMode='cover'
        />
        <View style={styles.overlay} />

        <View style={styles.content}>
          {/* Main Content Section with Poster */}
          <View style={styles.mainContent}>
            {/* Left Side - Poster */}
            <View style={styles.posterContainer}>
              <Image
                source={{ uri: movie.thumb }}
                style={styles.poster}
                resizeMode='cover'
              />

              {/* Button Group */}
              <View style={styles.buttonGroup}>
                <Pressable
                  focusable
                  style={({ focused }) => [
                    styles.button,
                    styles.playButton,
                    focused && styles.buttonFocused,
                  ]}
                  onPress={() => setIsPlaying(true)}
                >
                  <Text style={styles.buttonText}>Play</Text>
                </Pressable>

                <Pressable
                  focusable
                  style={({ focused }) => [
                    styles.button,
                    focused && styles.buttonFocused,
                  ]}
                  onPress={() => {
                    /* Handle mark as watched */
                  }}
                >
                  <Text style={styles.buttonText}>Mark as Watched</Text>
                </Pressable>

                <Pressable
                  focusable
                  style={({ focused }) => [
                    styles.button,
                    focused && styles.buttonFocused,
                  ]}
                  onPress={() => {
                    /* Handle more options */
                  }}
                >
                  <Text style={styles.buttonText}>More...</Text>
                </Pressable>
              </View>
            </View>

            {/* Right Side - Movie Info */}
            <View style={styles.infoContainer}>
              {/* Title and Tagline */}
              <View style={styles.headerSection}>
                <Text style={styles.title}>{movie.title}</Text>
                {movie.tagLine && (
                  <Text style={styles.tagline}>{movie.tagLine}</Text>
                )}
              </View>

              {/* Quick Info Section */}
              <View style={styles.quickInfoSection}>
                <Text style={styles.year}>
                  {new Date(movie.releaseDate).getFullYear()}
                </Text>
                <Text style={styles.runtime}>
                  {Math.floor(movie.runTime / 60)}h {movie.runTime % 60}m
                </Text>
                <Text style={styles.rating}>{movie.contentRating}</Text>
              </View>

              {/* Ratings Section */}
              <View style={styles.ratingsSection}>
                <View style={styles.ratingItem}>
                  <Text style={styles.ratingTitle}>Audience</Text>
                  <Text style={styles.ratingValue}>
                    {movie.audienceRating}%
                  </Text>
                </View>
                <View style={styles.ratingItem}>
                  <Text style={styles.ratingTitle}>Critics</Text>
                  <Text style={styles.ratingValue}>{movie.criticRating}%</Text>
                </View>
              </View>

              {/* Summary Section */}
              <View style={styles.section}>
                <Text style={styles.summary}>{movie.summary}</Text>
              </View>

              {/* Genres Section */}
              <View style={styles.section}>
                <View style={styles.genreContainer}>
                  {movie.genres.map(genre => (
                    <View key={genre.ID} style={styles.genreTag}>
                      <Text style={styles.genreText}>{genre.tag}</Text>
                    </View>
                  ))}
                </View>
              </View>
            </View>
          </View>

          {/* Cast Section */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Cast</Text>
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              style={styles.castContainer}
            >
              {movie.castList.map(cast => (
                <Pressable
                  key={cast.ID}
                  focusable
                  style={({ focused }) => [
                    styles.castItem,
                    focused && styles.itemFocused,
                  ]}
                >
                  <Image
                    source={{ uri: cast.artist.thumb }}
                    style={styles.artistImage}
                    resizeMode='cover'
                  />
                  <View style={styles.artistInfo}>
                    <Text style={styles.castName}>{cast.artist.name}</Text>
                    <Text style={styles.castCharacter}>{cast.character}</Text>
                  </View>
                </Pressable>
              ))}
            </ScrollView>
          </View>

          {/* Crew Section */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Crew</Text>
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              style={styles.crewContainer}
            >
              {movie.crewList.map(crew => (
                <Pressable
                  key={crew.ID}
                  focusable
                  style={({ focused }) => [
                    styles.crewItem,
                    focused && styles.itemFocused,
                  ]}
                >
                  <Image
                    source={{ uri: crew.artist.thumb }}
                    style={styles.artistImage}
                    resizeMode='cover'
                  />
                  <View style={styles.artistInfo}>
                    <Text style={styles.crewName}>{crew.artist.name}</Text>
                    <Text style={styles.crewJob}>{crew.job}</Text>
                  </View>
                </Pressable>
              ))}
            </ScrollView>
          </View>

          {/* Additional Info */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Additional Information</Text>
            <Text style={styles.infoText}>Studio: {movie.studios.name}</Text>
            <Text style={styles.infoText}>
              Budget: ${movie.budget.toLocaleString()}
            </Text>
            <Text style={styles.infoText}>
              Revenue: ${movie.revenue.toLocaleString()}
            </Text>
            <Text style={styles.infoText}>
              Release Date: {new Date(movie.releaseDate).toLocaleDateString()}
            </Text>
          </View>

          {/* Technical Details */}
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Technical Details</Text>
            <Text style={styles.infoText}>Resolution: {movie.resolution}p</Text>
            <Text style={styles.infoText}>
              Audio: {movie.audioList.map(audio => audio.language).join(", ")}
            </Text>
            <Text style={styles.infoText}>
              Subtitles:{" "}
              {movie.subtitleList.map(sub => sub.language).join(", ")}
            </Text>
          </View>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  container: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  backgroundArt: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    height: 400,
  },
  overlay: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    height: 400,
    backgroundColor: "rgba(18, 31, 50, 0.8)", // Matches your primary color
  },
  content: {
    padding: 20,
    marginTop: 300, // Adjust based on your needs
  },
  mainContent: {
    flexDirection: "row",
    gap: 40,
    marginBottom: 40,
  },
  posterContainer: {
    width: 300,
  },
  poster: {
    width: "100%",
    aspectRatio: 2 / 3,
    borderRadius: 8,
    backgroundColor: Colors.secondary,
    marginBottom: 20,
  },
  buttonGroup: {
    gap: 12,
  },
  button: {
    backgroundColor: Colors.secondary,
    paddingVertical: 12,
    paddingHorizontal: 20,
    borderRadius: 6,
    alignItems: "center",
    borderWidth: 2,
    borderColor: "transparent",
  },
  playButton: {
    backgroundColor: Colors.accent,
  },
  buttonFocused: {
    borderColor: Colors.accent,
    transform: [{ scale: 1.05 }],
  },
  buttonText: {
    color: Colors.textPrimary,
    fontSize: 18,
    fontWeight: "bold",
  },
  infoContainer: {
    flex: 1,
    paddingTop: 20,
  },
  headerSection: {
    marginBottom: 20,
  },
  title: {
    fontSize: 48,
    fontWeight: "bold",
    color: Colors.textPrimary,
    marginBottom: 8,
  },
  tagline: {
    fontSize: 24,
    color: Colors.textSecondary,
    fontStyle: "italic",
  },
  quickInfoSection: {
    flexDirection: "row",
    gap: 20,
    marginBottom: 20,
  },
  year: {
    fontSize: 20,
    color: Colors.textSecondary,
  },
  runtime: {
    fontSize: 20,
    color: Colors.textSecondary,
  },
  rating: {
    fontSize: 20,
    color: Colors.textSecondary,
  },
  ratingsSection: {
    flexDirection: "row",
    gap: 40,
    marginBottom: 30,
  },
  ratingItem: {
    alignItems: "center",
  },
  ratingTitle: {
    fontSize: 18,
    color: Colors.textSecondary,
  },
  ratingValue: {
    fontSize: 24,
    color: Colors.textPrimary,
    fontWeight: "bold",
  },
  section: {
    marginBottom: 30,
  },
  sectionTitle: {
    fontSize: 28,
    color: Colors.textPrimary,
    fontWeight: "bold",
    marginBottom: 15,
  },
  summary: {
    fontSize: 20,
    color: Colors.textSecondary,
    lineHeight: 30,
  },
  genreContainer: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 10,
  },
  genreTag: {
    backgroundColor: Colors.secondary,
    paddingVertical: 8,
    paddingHorizontal: 16,
    borderRadius: 20,
  },
  genreText: {
    color: Colors.textPrimary,
    fontSize: 18,
  },
  castContainer: {
    marginTop: 10,
  },
  castItem: {
    width: 160,
    marginRight: 16,
    backgroundColor: Colors.cardBackground,
    borderRadius: 8,
    overflow: "hidden",
    borderWidth: 2,
    borderColor: "transparent",
  },
  crewContainer: {
    marginTop: 10,
  },
  crewItem: {
    width: 160,
    marginRight: 16,
    backgroundColor: Colors.cardBackground,
    borderRadius: 8,
    overflow: "hidden",
    borderWidth: 2,
    borderColor: "transparent",
  },
  itemFocused: {
    borderColor: Colors.accent,
    backgroundColor: Colors.secondary,
    transform: [{ scale: 1.1 }],
  },
  artistImage: {
    width: "100%",
    height: 200,
    backgroundColor: Colors.secondary,
  },
  artistInfo: {
    padding: 12,
  },
  castName: {
    fontSize: 16,
    color: Colors.textPrimary,
    fontWeight: "bold",
    marginBottom: 4,
  },
  castCharacter: {
    fontSize: 14,
    color: Colors.textSecondary,
  },
  crewName: {
    fontSize: 16,
    color: Colors.textPrimary,
    fontWeight: "bold",
    marginBottom: 4,
  },
  crewJob: {
    fontSize: 14,
    color: Colors.textSecondary,
  },
  infoText: {
    fontSize: 18,
    color: Colors.textSecondary,
    marginBottom: 8,
  },
});
