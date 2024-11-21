import { View, Text, Pressable } from "react-native";
import { FlashList } from "@shopify/flash-list";
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
    queryFn: async ({ pageParam = "" }) => {
      const { data } = await api.get<MoviesResponse>("/auth/movies", {
        params: {
          cursor: pageParam,
        },
      });
      return data;
    },
    initialPageParam: "",
    getNextPageParam: lastPage => lastPage.nextCursor,
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

  const allMovies =
    data?.pages.flatMap((page: MoviesResponse) => page.movies) ?? [];

  return (
    <View className='flex-1 bg-dark'>
      {/* Header */}
      <View className='px-8 py-6'>
        <Text className='text-light text-4xl font-bold'>Movies</Text>
      </View>

      {/* Movies Grid */}
      <FlashList
        data={allMovies}
        numColumns={6}
        estimatedItemSize={300}
        keyExtractor={item => item.ID.toString()}
        renderItem={({ item: movie, index }) => (
          <View className='p-3'>
            <MovieCard movie={movie} hasTVPreferredFocus={index === 0} />
          </View>
        )}
        onEndReached={() => {
          if (hasNextPage && !isFetchingNextPage) {
            fetchNextPage();
          }
        }}
        onEndReachedThreshold={0.5}
        ListFooterComponent={() =>
          hasNextPage ? (
            <View className='p-3'>
              <Pressable
                className={`
                  h-[300px] w-[200px] rounded-lg 
                  justify-center items-center
                  bg-primary/20
                  focus:bg-secondary/20 focus:scale-110
                `}
                focusable={!isFetchingNextPage}
              >
                <Text className='text-light text-xl text-center'>
                  {isFetchingNextPage ? "Loading..." : "Load More"}
                </Text>
              </Pressable>
            </View>
          ) : null
        }
        contentContainerStyle={{ paddingHorizontal: 32 }}
      />
    </View>
  );
}
