import { View, ScrollView, Text } from "react-native";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

export default function LatestMovies() {
  const {
    data: movies,
    isLoading,
    isError,
  } = useQuery<SimpleMovie[]>({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const res = await api.get("/latest-movies");

        if (!res.data.movies) {
          throw new Error("unable to fetch latest movies");
        }

        return res.data.movies;
      } catch (error) {
        console.error("Error fetching movies:", error);
        throw new Error("unable to fetch movies");
      }
    },
  });

  if (isLoading) {
    return (
      <View className='p-8'>
        <Text className='text-info text-2xl'>Loading latest movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View className='p-8'>
        <Text className='text-danger text-2xl'>Error loading movies</Text>
      </View>
    );
  }

  return (
    <View className='py-8'>
      {/* Section Title */}
      <Text className='text-light text-2xl font-bold px-8 mb-6'>
        Latest Movies
      </Text>

      {/* Movies Grid */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        className='px-8'
      >
        <View className='flex-row gap-6'>
          {movies?.map((movie, index) => (
            <View key={movie.ID} className='w-[200px]'>
              <MovieCard movie={movie} hasTVPreferredFocus={index === 0} />
            </View>
          ))}
        </View>
      </ScrollView>
    </View>
  );
}
