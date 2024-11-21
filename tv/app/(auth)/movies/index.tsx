import { View, Text, Pressable } from "react-native";
import { useInfiniteQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";
import { AxiosError } from "axios";

type MoviesResponse = {
  movies: SimpleMovie[];
  nextCursor?: string;
};

export default function MoviesScreen() {
  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    status,
    error,
  } = useInfiniteQuery({
    queryKey: ["movies"],
    queryFn: async ({ pageParam }) => {
      const { data } = await api.get<MoviesResponse>("/auth/movies", {
        params: { cursor: pageParam },
      });
      return data;
    },
    initialPageParam: "",
    getNextPageParam: (lastPage: MoviesResponse) => lastPage.nextCursor,
  });

  if (status === "pending") {
    return (
      <View className='flex-1 p-8'>
        <Text className='text-info text-2xl'>Loading movies...</Text>
      </View>
    );
  }

  if (status === "error") {
    const errorMessage =
      error instanceof AxiosError
        ? error.response?.data?.error || error.message
        : "Error loading movies";

    return (
      <View className='flex-1 p-8'>
        <Text className='text-danger text-2xl'>{errorMessage}</Text>
      </View>
    );
  }

  if (!data) {
    return null;
  }

  const allMovies = data.pages.flatMap(page => page.movies);

  return (
    <View className='flex-1 bg-dark p-8'>
      {/* Header */}
      <Text className='text-light text-4xl font-bold mb-8'>Movies</Text>

      {/* Movies Grid */}
      <View className='flex-row flex-wrap gap-6'>
        {allMovies.map((movie, index) => (
          <View key={movie.ID}>
            <MovieCard movie={movie} hasTVPreferredFocus={index === 0} />
          </View>
        ))}

        {/* Load More - Triggered when last item is focused */}
        {hasNextPage && (
          <Pressable
            className={`
              h-[300px] w-[200px] rounded-lg 
              justify-center items-center
              bg-primary/20
              focus:bg-secondary/20 focus:scale-110
            `}
            focusable={!isFetchingNextPage}
            onFocus={() => {
              if (!isFetchingNextPage) {
                fetchNextPage();
              }
            }}
          >
            <Text className='text-light text-xl text-center'>
              {isFetchingNextPage ? "Loading..." : "Load More"}
            </Text>
          </Pressable>
        )}
      </View>
    </View>
  );
}
