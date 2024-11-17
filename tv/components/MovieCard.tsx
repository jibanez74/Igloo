import { View, Text, Image, Pressable } from "react-native";
import { useRouter } from "expo-router";
import type { SimpleMovie } from "@/types/Movie";

export default function MovieCard({
  movie,
  hasTVPreferredFocus = false,
}: {
  movie: SimpleMovie;
  hasTVPreferredFocus?: boolean;
}) {
  const router = useRouter();

  return (
    <Pressable
      className='rounded-lg overflow-hidden bg-primary transform transition-transform duration-200
                focus:scale-110 focus:bg-secondary focus:border-2 focus:border-light'
      focusable={true}
      hasTVPreferredFocus={hasTVPreferredFocus}
      nextFocusUp={undefined}
      nextFocusDown={undefined}
      nextFocusLeft={undefined}
      nextFocusRight={undefined}
    >
      {/* Thumbnail */}
      <View className='relative'>
        <Image
          source={{ uri: movie.thumb }}
          className='w-full aspect-[2/3]'
          resizeMode='cover'
        />

        {/* Gradient Overlay */}
        <View
          className='absolute bottom-0 left-0 right-0 h-24 
                    bg-gradient-to-t from-dark/90 to-transparent'
        />
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
