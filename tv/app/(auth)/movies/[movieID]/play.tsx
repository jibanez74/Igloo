import { View } from "react-native";
import { useRouter, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import VideoPlayer from "@/components/VideoPlayer";
import api from "@/lib/api";
import getImgSrc from "@/lib/getImgSrc";
import type { Movie } from "@/types/Movie";

type MovieResponse = {
  movie: Movie;
};

export default function PlayMovie() {
  const { movieID } = useLocalSearchParams<{ movieID: string }>();

  const router = useRouter();

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

  return (
    movie &&
    !isLoading &&
    !isError && (
      <View className='flex-1'>
        <VideoPlayer
          contentType={movie.contentType}
          resolution={2160}
          size={movie.size}
          thumb={getImgSrc(movie.thumb)}
          uri={`${process.env.EXPO_PUBLIC_API_URL}/api/v1/auth/movies/stream/direct/${movie.ID}`}
          onClose={() => router.back()}
        />
      </View>
    )
  );
}
