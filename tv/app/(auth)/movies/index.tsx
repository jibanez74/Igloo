import { View, Text, Pressable } from "react-native";
import { FlashList } from "@shopify/flash-list";
import { useInfiniteQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import Container from "@/components/Container";
import api from "@/lib/api";
import { AxiosError } from "axios";
import type { SimpleMovie } from "@/types/Movie";

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
      const { data } = await api.get<MoviesResponse>("/auth/movies/infinite", {
        params: pageParam ? { cursor: pageParam } : undefined,
      });

      return data;
    },
    initialPageParam: "",
    getNextPageParam: lastPage => lastPage.nextCursor,
  });

  if (status === "pending") {
    return (
      <View className='flex-1 bg-dark'>
        <Container>
          <Text className='text-info text-2xl'>Loading movies...</Text>
        </Container>
      </View>
    );
  }

  if (status === "error") {
    const errorMessage =
      error instanceof AxiosError
        ? error.response?.data?.error || error.message
        : "Error loading movies";

    return (
      <View className='flex-1 bg-dark'>
        <Container>
          <Text className='text-danger text-2xl'>{errorMessage}</Text>
        </Container>
      </View>
    );
  }

  const allMovies = data?.pages.flatMap(page => page.movies) ?? [];

  return (
    <View className='flex-1 bg-dark'>
      <Container>
        <Text className='text-light text-4xl font-bold py-6'>All Movies</Text>

        <View className='flex-1 h-full'>
          <FlashList
            data={allMovies}
            numColumns={6}
            estimatedItemSize={350}
            keyExtractor={(item: SimpleMovie) => item.ID.toString()}
            renderItem={({ item, index }) => (
              <View className='p-3 w-1/6'>
                <MovieCard movie={item} hasTVPreferredFocus={index === 0} />
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
            contentContainerStyle={{
              paddingBottom: 32,
            }}
          />
        </View>
      </Container>
    </View>
  );
}
