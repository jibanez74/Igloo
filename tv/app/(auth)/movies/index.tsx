import { View, Text } from "react-native";
import { FlashList } from "@shopify/flash-list";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import { TV_LAYOUT } from "@/constants/tv";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

type MoviesResponse = {
  movies: SimpleMovie[];
  count: number;
};

export default function MoviesScreen() {
  const { data, error, isError, isPending } = useQuery({
    queryKey: ["movies"],
    queryFn: async () => {
      const { data } = await api.get<MoviesResponse>("/auth/movies/all");
      return data;
    },
    staleTime: 5 * 60 * 1000,
  });

  const skeletonData = Array(24).fill(null);

  return (
    <View className='flex-1 bg-dark'>
      {/* Header */}
      <View className='px-8 py-6'>
        <Text className='text-light text-4xl font-bold'>
          All Movies ({!isPending ? data?.count || 0 : "..."})
        </Text>
      </View>

      {/* Grid Container */}
      <View className='flex-1 px-8'>
        <FlashList
          data={isPending ? skeletonData : data?.movies}
          numColumns={TV_LAYOUT.GRID.columns}
          estimatedItemSize={
            TV_LAYOUT.POSTER.HEIGHT + TV_LAYOUT.GRID.SPACING * 2
          }
          keyExtractor={(item, index) =>
            item?.ID?.toString() || `skeleton-${index}`
          }
          renderItem={({ item, index }) => (
            <View
              style={{
                width: TV_LAYOUT.POSTER.WIDTH,
                marginHorizontal: TV_LAYOUT.GRID.SPACING / 2,
                marginVertical: TV_LAYOUT.GRID.SPACING,
              }}
            >
              {isPending ? (
                <View>
                  <View className='w-full aspect-[2/3] rounded-lg bg-primary/30 animate-pulse' />
                  <View className='h-6 w-3/4 mt-4 bg-primary/30 rounded animate-pulse' />
                  <View className='h-4 w-1/2 mt-2 bg-primary/30 rounded animate-pulse' />
                </View>
              ) : (
                <MovieCard
                  movie={item}
                  hasTVPreferredFocus={index === 0}
                  width={TV_LAYOUT.POSTER.WIDTH}
                  height={TV_LAYOUT.POSTER.HEIGHT}
                />
              )}
            </View>
          )}
        />
      </View>

      {/* Error Overlay */}
      {isError && (
        <View className='absolute inset-0 flex items-center justify-center bg-dark/80'>
          <Text className='text-danger text-2xl'>
            {error?.message || "Error loading movies"}
          </Text>
        </View>
      )}
    </View>
  );
}
