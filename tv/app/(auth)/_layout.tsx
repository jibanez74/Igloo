import { useState } from "react";
import { View, Text, Pressable } from "react-native";
import { Slot, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

export default function AuthLayout() {
  const [activeRoute, setActiveRoute] = useState("index");

  return (
    <View className='flex-1 bg-dark'>
      {/* Top Navigation Bar */}
      <View className='flex-row items-center h-[90px] px-8 bg-gradient-to-b from-dark via-dark/80 to-transparent'>
        {/* Home Button */}
        <Pressable
          focusable={true}
          hasTVPreferredFocus={true}
          onPress={() => {
            setActiveRoute("index");
          }}
          className={`
            flex-row items-center px-8 h-[90px] mr-4
            rounded-lg overflow-hidden transition-all duration-200
            ${activeRoute === "index" ? "bg-secondary/20" : ""}
            focus:bg-secondary/30 focus:scale-110
          `}
        >
          <Ionicons
            name={activeRoute === "index" ? "home" : "home-outline"}
            size={32}
            color={activeRoute === "index" ? "#CEE3F9" : "#6B7280"}
          />
          <Text
            className={`
              ml-3 text-xl font-semibold
              ${activeRoute === "index" ? "text-light" : "text-info/60"}
            `}
          >
            Home
          </Text>
        </Pressable>

        {/* Movies Button */}
        <Pressable
          focusable={true}
          onPress={() => {
            setActiveRoute("movies");
          }}
          className={`
            flex-row items-center px-8 h-[90px] mr-4
            rounded-lg overflow-hidden transition-all duration-200
            ${activeRoute === "movies" ? "bg-secondary/20" : ""}
            focus:bg-secondary/30 focus:scale-110
          `}
        >
          <Ionicons
            name={activeRoute === "movies" ? "film" : "film-outline"}
            size={32}
            color={activeRoute === "movies" ? "#CEE3F9" : "#6B7280"}
          />
          <Text
            className={`
              ml-3 text-xl font-semibold
              ${activeRoute === "movies" ? "text-light" : "text-info/60"}
            `}
          >
            Movies
          </Text>
        </Pressable>
      </View>

      {/* Content Area */}
      <View className='flex-1'>
        <Slot />
      </View>
    </View>
  );
}
