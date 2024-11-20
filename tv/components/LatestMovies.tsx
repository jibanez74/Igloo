import { View, Text } from "react-native";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";
import type { ErrorResponse } from "@/types/ErrorResponse";

type MoviesResponse = {
  movies: SimpleMovie[];
};

export default function LatestMovies() {
  const { data, isPending, isError, error } = useQuery<
    SimpleMovie[],
    Error & { cause?: ErrorResponse }
  >({
    queryKey: ["latest-movies"],
    queryFn: async () => {
      try {
        const res = await api.get<MoviesResponse>("/latest-movies");
        if (!res.movies) {
          throw new Error("No movies found");
        }
        return res.movies.slice(0, 12);
      } catch (err: any) {
        const apiError = err.cause?.error || err.message;
        throw new Error(apiError);
      }
    },
  });

  return (
    <View className='py-8'>
      {/* Section Title */}
      <Text className='text-light text-3xl font-bold mb-6'>Latest Movies</Text>

      {/* Movies Row */}
      <View className='flex-row gap-6'>
        {isPending ? (
          // Loading placeholders
          Array.from({ length: 6 }).map((_, index) => (
            <View
              key={`skeleton-${index}`}
              className='w-[200px] rounded-lg overflow-hidden bg-primary/20'
            >
              {/* Thumbnail skeleton */}
              <View className='aspect-[2/3] bg-primary/30 animate-pulse' />

              {/* Content skeleton */}
              <View className='p-4'>
                <View className='h-6 w-3/4 bg-primary/30 mb-2 rounded animate-pulse' />
                <View className='h-4 w-1/2 bg-primary/30 rounded animate-pulse' />
              </View>
            </View>
          ))
        ) : isError ? (
          <View className='p-8'>
            <Text className='text-danger text-2xl'>
              {error.message || "Error loading movies"}
            </Text>
          </View>
        ) : (
          data?.map((movie, index) => (
            <View key={movie.ID}>
              <MovieCard movie={movie} hasTVPreferredFocus={index === 0} />
            </View>
          ))
        )}
      </View>
    </View>
  );
}
