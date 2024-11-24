import { View, Text, Image, Pressable } from "react-native";
import { useLocalSearchParams, router } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Ionicons } from "@expo/vector-icons";
import api from "@/lib/api";
import formatDate from "@/lib/formatDate";
import type { Movie } from "@/types/Movie";

type MovieResponse = {
  movie: Movie;
};

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams<{ movieID: string }>();

  const {
    data: movie,
    isLoading,
    error,
    isError,
  } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: async () => {
      const { data } = await api.get<MovieResponse>(`/auth/movies/${movieID}`);

      if (!data.movie) {
        throw new Error("Movie not found");
      }

      return data.movie;
    },
  });

  if (isLoading) {
    return (
      <View className='flex-1 bg-dark p-8'>
        <Text className='text-info text-2xl'>Loading movie details...</Text>
      </View>
    );
  }

  if (error || !movie) {
    return (
      <View className='flex-1 bg-dark p-8'>
        <Text className='text-danger text-2xl'>
          Error loading movie details
        </Text>
      </View>
    );
  }

  return (
    <View className='flex-1 bg-dark'>
      {/* Back Button */}
      <Pressable
        className='absolute top-8 left-8 z-10 flex-row items-center
                  bg-dark/80 rounded-lg px-4 py-2
                  focus:bg-secondary/20 focus:scale-110'
        focusable={true}
        onPress={() => router.back()}
      >
        <Ionicons name='arrow-back' size={24} color='#CEE3F9' />
        <Text className='text-light text-xl ml-2'>Back</Text>
      </Pressable>

      {/* Main Content */}
      <View className='flex-1 flex-row'>
        {/* Left Side - Backdrop and Main Info */}
        <View className='w-2/3 h-full relative'>
          <Image
            source={{ uri: movie.art }}
            className='w-full h-full'
            resizeMode='cover'
          />
          <View className='absolute inset-0 bg-gradient-to-r from-dark via-dark/50 to-transparent' />

          {/* Content Overlay */}
          <View className='absolute bottom-0 left-a0 p-8 w-full'>
            <Text className='text-light text-6xl font-bold mb-4'>
              {movie.title}
            </Text>

            {movie.tagLine && (
              <Text className='text-secondary text-2xl italic mb-4'>
                {movie.tagLine}
              </Text>
            )}

            {/* Meta Info */}
            <View className='flex-row items-center gap-4 mb-4'>
              <Text className='text-info text-xl'>{movie.year}</Text>
              <Text className='text-info text-xl'>{movie.contentRating}</Text>
              <Text className='text-info text-xl'>
                {Math.floor(movie.runTime / 60)}h {movie.runTime % 60}m
              </Text>
              <Text className='text-info text-xl'>
                Released {formatDate(movie.releaseDate)}
              </Text>
            </View>

            {/* Play Button */}
            <Pressable
              className='bg-secondary py-4 px-8 rounded-lg w-[200px] items-center mt-6
                        focus:scale-110 focus:bg-info'
              focusable={true}
              hasTVPreferredFocus={true}
            >
              <Text className='text-dark text-xl font-bold'>Play</Text>
            </Pressable>
          </View>
        </View>

        {/* Right Side - Details */}
        <View className='w-1/3 p-8 overflow-y-auto'>
          {/* Summary */}
          <Text className='text-light text-xl mb-6'>{movie.summary}</Text>

          {/* Ratings */}
          {movie.audienceRating && (
            <View className='flex-row items-center mb-6'>
              <Ionicons name='star' size={24} color='#FDAC00' />
              <Text className='text-light text-xl ml-2'>
                {movie.audienceRating.toFixed(1)}
              </Text>
            </View>
          )}

          {/* Financial Details */}
          <View className='flex-row gap-8 mb-6'>
            {movie.budget > 0 && (
              <View>
                <Text className='text-info/60 text-lg'>Budget</Text>
                <Text className='text-light text-xl'>
                  ${movie.budget.toLocaleString()}
                </Text>
              </View>
            )}
            {movie.revenue > 0 && (
              <View>
                <Text className='text-info/60 text-lg'>Box Office</Text>
                <Text className='text-light text-xl'>
                  ${movie.revenue.toLocaleString()}
                </Text>
              </View>
            )}
          </View>

          {/* Cast & Crew */}
          {movie.castList.length > 0 && (
            <View className='mb-6'>
              <Text className='text-light text-2xl font-bold mb-2'>Cast</Text>
              {movie.castList.slice(0, 6).map(cast => (
                <Text key={cast.ID} className='text-info text-lg'>
                  {cast.artist.name} as {cast.character}
                </Text>
              ))}
            </View>
          )}

          {/* Genres */}
          {movie.genres.length > 0 && (
            <View className='mb-6'>
              <Text className='text-light text-2xl font-bold mb-2'>Genres</Text>
              <View className='flex-row flex-wrap gap-2'>
                {movie.genres.map(genre => (
                  <Text key={genre.ID} className='text-info text-lg'>
                    {genre.title}
                  </Text>
                ))}
              </View>
            </View>
          )}
        </View>
      </View>
    </View>
  );
}
