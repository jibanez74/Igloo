import { View, Text, Pressable, Image } from "react-native";
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
  // Construct the full URL for the image
  const imageUrl = movie.thumb.startsWith("http")
    ? movie.thumb
    : `${process.env.EXPO_PUBLIC_API_URL}${movie.thumb}`;

  return (
    <Pressable
      style={{
        width,
        borderRadius: 8,
        overflow: "hidden",
        backgroundColor: "rgba(28, 57, 94, 0.2)",
        transform: [], // Required for TV focus animation
      }}
      className={`
        transform transition-all duration-200
        focus:scale-110 focus:bg-secondary/20
        focus:ring-2 focus:ring-secondary
      `}
      focusable={true}
      hasTVPreferredFocus={hasTVPreferredFocus}
    >
      {/* Thumbnail */}
      <View>
        <Image
          source={{ uri: imageUrl }}
          style={{
            width,
            height,
          }}
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
