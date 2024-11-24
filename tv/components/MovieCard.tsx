import { View, Text, Pressable, Image } from "react-native";
import { useRouter } from "expo-router";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  hasTVPreferredFocus?: boolean;
  width: number;
  height: number;
};

export default function MovieCard({
  movie,
  hasTVPreferredFocus = false,
  width,
  height,
}: MovieCardProps) {
  const imageUrl = movie.thumb.startsWith("http")
    ? movie.thumb
    : `${process.env.EXPO_PUBLIC_API_URL}${movie.thumb}`;

  const router = useRouter();

  return (
    <Pressable
      className={`
        rounded-lg overflow-hidden bg-primary/20
        transform transition-all duration-200
        focus:scale-110 focus:bg-secondary/20
        focus:ring-2 focus:ring-secondary
      `}
      style={{ width }}
      focusable={true}
      hasTVPreferredFocus={hasTVPreferredFocus}
      onPress={() =>
        router.push({
          pathname: "/(auth)/movies/[movieID]",
          params: { movieID: movie.ID },
        })
      }
    >
      {/* Thumbnail */}
      <View className='relative'>
        <Image
          source={{ uri: imageUrl }}
          className='w-full'
          style={{ height }}
          resizeMode='cover'
        />

        {/* Gradient Overlay */}
        <View className='absolute inset-0 bg-gradient-to-t from-dark/90 to-transparent' />
      </View>

      {/* Content */}
      <View className='p-4'>
        <Text className='text-light text-xl font-bold mb-2' numberOfLines={1}>
          {movie.title}
        </Text>
        <Text className='text-info text-lg' numberOfLines={1}>
          {movie.year}
        </Text>
      </View>
    </Pressable>
  );
}
