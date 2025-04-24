import { View, Text, Pressable } from "react-native";
import { useRouter } from "expo-router";
import { Image } from "expo-image";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  index: number;
};

export default function MovieCard({ movie, index }: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);

  const router = useRouter();

  return (
    <Pressable
      onPress={() =>
        router.navigate({
          pathname: "/(tabs)/movies/[movieID]",
          params: {
            movieID: movie.id.toString(),
          },
        })
      }
      hasTVPreferredFocus={index === 0}
    >
      <View>
        <View>
          <Image
            source={imgSrc}
            contentFit="cover"
            cachePolicy="memory-disk"
            allowDownscaling
          />
        </View>

        <View>
          <Text>{movie.title}</Text>

          <Text>{movie.year}</Text>
        </View>
      </View>
    </Pressable>
  );
}
