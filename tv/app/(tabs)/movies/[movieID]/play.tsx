import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import getImgSrc from "@/lib/getImgSrc";
import API_URL from "@/constants/Backend";
import TvVideoPlayer from "@/components/VideoPlayer";
import type { Movie } from "@/types/Movie";

export default function PlayMovieScreen() {
  const { movieID } = useLocalSearchParams();

  const {
    isPending,
    data,
    isError,
    error,
  } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: async (): Promise<Movie> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieID}/details`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movie;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movie");
      }
    },
    refetchOnWindowFocus: false,
  });

  if (isPending) {
    return (
      <View>
        <Text>Movie is loading...</Text>
      </View>
    );
  }

  return (
    <View>
      {data && (
        <TvVideoPlayer
          thumb={getImgSrc(data.thumb)}
          title={data.title}
          videoUri={`${API_URL}/movies${movieID}/stream`}
        />
      )}
    </View>
  );
}
