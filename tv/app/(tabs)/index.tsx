import React from "react";
import { View, Text, Pressable, Image } from "react-native";
import { MaterialCommunityIcons } from "@expo/vector-icons";
import LatestMovies from "@/components/LatestMovies";

export default function HomeScreen() {
  return (
    <View className="flex-1">
      <View className="relative h-[60vh] items-center justify-center overflow-hidden">
        <View className="absolute inset-0 bg-blue-950/90" />

        <View className="relative z-10 items-center px-4">
          <View className="max-w-3xl items-center space-y-6">
            <MaterialCommunityIcons
              name="film"
              size={64}
              color="#fbbf24"
              className="mx-auto"
            />

            <Text className="text-4xl font-bold text-center">
              <Text className="text-yellow-300">Welcome to Igloo</Text>
            </Text>

            <Text className="text-xl text-blue-200 text-center">
              Your personal media server for Movies, TV Shows, Music, and more
            </Text>

            <View className="flex-row justify-center gap-4">
              <Pressable
                className="flex-row items-center gap-2 px-6 py-3 rounded-lg bg-blue-600"
              >
                <MaterialCommunityIcons
                  name="play"
                  size={20}
                  color="white"
                />
                <Text className="text-white">Start Watching</Text>
              </Pressable>
            </View>
          </View>
        </View>
      </View>

      <View className="bg-transparent">
        <LatestMovies />
      </View>
    </View>
  );
}
