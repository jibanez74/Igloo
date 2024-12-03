import { View, Text } from "react-native";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import { TV_LAYOUT } from "@/constants/tv";
import api from "@/lib/api";
import type { MoviesResponse } from "@/types/Movie";

export default function LatestMovies() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/auth/movies/latest");

        if (!data.movies) {
          throw new Error("no movies were fetched");
        }

        return data.movies;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movies from server");
      }
    },
  });

  if (isPending) {
    return (
      <View className='py-8'>
        <Text className='text-info text-2xl'>Loading latest movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View className='py-8'>
        <Text className='text-danger text-2xl'>
          {error?.message || "Error loading latest movies"}
        </Text>
      </View>
    );
  }

  return (
    <View className='py-8'>
      <Text className='text-light text-3xl font-bold mb-6'>Latest Movies</Text>

      <View className='flex-row gap-6'>
        {data.map((movie, index) => (
          <View key={movie.ID}>
            <MovieCard
              movie={movie}
              hasTVPreferredFocus={index === 0}
              width={TV_LAYOUT.POSTER.WIDTH}
              height={TV_LAYOUT.POSTER.HEIGHT}
            />
          </View>
        ))}
      </View>
    </View>
  );
}
