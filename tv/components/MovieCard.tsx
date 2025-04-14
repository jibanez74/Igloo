import { View, Text, Pressable, useTVEventHandler } from "react-native";
import { Image } from "expo-image";
import { Link } from "expo-router";
import { useState } from "react";
import type { SimpleMovie } from "@/types/Movie";
import getImgSrc from "@/utils/getImgSrc";
type MovieCardProps = {
  movie: SimpleMovie;
};

export default function MovieCard({ movie }: MovieCardProps) {
  const [isFocused, setIsFocused] = useState(false);

  useTVEventHandler((event) => {
    if (event.eventType === "focus") {
      setIsFocused(true);
    } else if (event.eventType === "blur") {
      setIsFocused(false);
    }
  });

  return (
    <Link href="/" asChild>
      <Pressable
        className={`group relative bg-blue-950/50 rounded-xl overflow-hidden shadow-lg shadow-blue-900/20 transition-all duration-300 ${
          isFocused ? "shadow-yellow-300/20 scale-[1.02]" : ""
        }`}
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
      >
        {({ pressed, focused }) => (
          <View className="w-full h-full">
            <Image
              source={{ uri: getImgSrc(movie.thumb) }}
              className="w-full h-full"
              contentFit="cover"
              transition={300}
              style={{
                transform: [{ scale: focused ? 1.05 : 1 }],
                opacity: focused ? 0.3 : 1,
              }}
            />

            <View
              className="absolute inset-0 bg-gradient-to-t from-blue-950 via-blue-950/50 to-transparent"
              style={{ opacity: focused ? 0.9 : 0.5 }}
            />

            <View className="absolute inset-0 p-4 flex flex-col justify-end">
              <View
                className="transform transition-all duration-300"
                style={{
                  transform: [{ translateY: focused ? 0 : 20 }],
                }}
              >
                <Text className="text-lg font-semibold text-white mb-2">
                  {movie.title}
                </Text>

                <Text className="text-sm text-yellow-300/90">{movie.year}</Text>
              </View>
            </View>
          </View>
        )}
      </Pressable>
    </Link>
  );
}
