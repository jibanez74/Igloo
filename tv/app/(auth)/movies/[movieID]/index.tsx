import React from "react";
import { View, Text, Image, Pressable } from "react-native";
import { useLocalSearchParams, router } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Ionicons } from "@expo/vector-icons";
import { LinearGradient } from "expo-linear-gradient";
import api from "@/lib/api";
import formatDate from "@/lib/formatDate";
import formatDollars from "@/lib/formatDollars";
import getImgSrc from "@/lib/getImgSrc";
import CastGrid from "@/components/CastGrid";
import StudioGrid from "@/lib/StudioGrid";
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
      if (!data.movie) throw new Error("Movie not found");
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

  if (isError || !movie) {
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
            source={{ uri: getImgSrc(movie.art) }}
            className='w-full h-full'
            resizeMode='cover'
          />
          <LinearGradient
            colors={[
              "rgba(18, 31, 50, 1)", // dark
              "rgba(18, 31, 50, 0.7)", // dark with opacity
              "transparent",
            ]}
            start={{ x: 0, y: 0 }}
            end={{ x: 1, y: 0 }}
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
            }}
          />

          {/* Content Overlay */}
          <View className='absolute bottom-0 left-0 p-8 w-full'>
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
              className='w-[200px] mt-6 overflow-hidden rounded-lg
                        focus:scale-110'
              focusable={true}
              hasTVPreferredFocus={true}
              onPress={() =>
                router.push({
                  pathname: "/(auth)/movies/[movieID]/play",
                  params: { movieID: movie.ID },
                })
              }
            >
              <LinearGradient
                colors={["#4F46E5", "#3730A3"]}
                start={{ x: 0, y: 0 }}
                end={{ x: 1, y: 1 }}
                style={{ padding: 16, alignItems: "center" }}
              >
                <Text className='text-light text-xl font-bold'>Play</Text>
              </LinearGradient>
            </Pressable>
          </View>
        </View>

        {/* Right Side - Details */}
        <View className='w-1/3 p-8 overflow-y-scroll'>
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

          {/* Cast Grid */}
          {movie.castList.length > 0 && (
            <CastGrid cast={movie.castList} maxDisplay={6} />
          )}

          {/* Studios Grid */}
          {movie.studios.length > 0 && (
            <StudioGrid studios={movie.studios} maxDisplay={6} />
          )}

          {/* Financial Details */}
          <View className='flex-row gap-8 mb-6'>
            {movie.budget > 0 && (
              <View>
                <Text className='text-info/60 text-lg'>Budget</Text>
                <Text className='text-light text-xl'>
                  {formatDollars(movie.budget)}
                </Text>
              </View>
            )}
            {movie.revenue > 0 && (
              <View>
                <Text className='text-info/60 text-lg'>Box Office</Text>
                <Text className='text-light text-xl'>
                  {formatDollars(movie.revenue)}
                </Text>
              </View>
            )}
          </View>

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
