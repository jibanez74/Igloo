import { View, Text, Pressable, Image } from "react-native";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  hasTVPreferredFocus?: boolean;
};

export default function MovieCard({
  movie,
  hasTVPreferredFocus = false,
}: MovieCardProps) {
  return (
    <Pressable
      className={`
        w-[200px] rounded-lg overflow-hidden bg-primary/20
        transform transition-all duration-200
        focus:scale-110 focus:bg-secondary/20
        focus:ring-2 focus:ring-secondary
      `}
      focusable={true}
      hasTVPreferredFocus={hasTVPreferredFocus}
    >
      {/* Thumbnail */}
      <View className='relative'>
        <Image
          source={{ uri: movie.thumb }}
          className='w-full aspect-[2/3]'
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
