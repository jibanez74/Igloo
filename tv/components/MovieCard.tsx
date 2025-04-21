import { View, Text, Pressable } from "react-native";
import { Link } from "expo-router";
import { Image } from "expo-image";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  index: number;
};

export default function MovieCard({ movie, index }: MovieCardProps) {
  const imgSrc = getImgSrc(movie.thumb);

  return (
    <Link
    href={{
        pathname: "/(tabs)/movies/[movieID]",
        params: {
            movieID: movie.id.toString()
        }
    }}
    asChild
    >
        <Pressable
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
    </Link>
  );
}
