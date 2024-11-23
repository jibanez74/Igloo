import { useEffect, useState } from "react";
import { View, Text, Pressable } from "react-native";
import { Slot, useRouter } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

export default function AuthLayout() {
  const router = useRouter();
  const [activeRoute, setActiveRoute] = useState("index");

  return (
    <View className='flex-1 bg-dark'>
      <View className='flex-row items-center justify-between h-[70px] px-12 bg-gradient-to-b from-dark via-dark/80 to-transparent'>
        {/* Left Group */}
        <View className='flex-row items-center gap-2'>
          {/* Home Button */}
          <Pressable
            focusable={true}
            hasTVPreferredFocus={true}
            onPress={() => {
              setActiveRoute("index");
              router.push("/(auth)/");
            }}
            className={`
              flex-row items-center px-8 h-[50px]
              rounded-lg overflow-hidden transition-all duration-200
              ${activeRoute === "index" ? "bg-secondary/20" : ""}
              focus:bg-secondary/30 focus:scale-110
            `}
          >
            <Ionicons
              name={activeRoute === "index" ? "home" : "home-outline"}
              size={28}
              color={activeRoute === "index" ? "#CEE3F9" : "#6B7280"}
            />
            <Text
              className={`
                ml-2 text-lg font-semibold  
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
              router.push("/(auth)/movies");
            }}
            className={`
              flex-row items-center px-8 h-[50px]
              rounded-lg overflow-hidden transition-all duration-200
              ${activeRoute === "movies" ? "bg-secondary/20" : ""}
              focus:bg-secondary/30 focus:scale-110
            `}
          >
            <Ionicons
              name={activeRoute === "movies" ? "film" : "film-outline"}
              size={28}
              color={activeRoute === "movies" ? "#CEE3F9" : "#6B7280"}
            />
            <Text
              className={`
                ml-2 text-lg font-semibold
                ${activeRoute === "movies" ? "text-light" : "text-info/60"}
              `}
            >
              Movies
            </Text>
          </Pressable>

          {/* TV Shows Button */}
          <Pressable
            focusable={true}
            onPress={() => {
              setActiveRoute("tv-shows");
            }}
            className={`
              flex-row items-center px-8 h-[50px]
              rounded-lg overflow-hidden transition-all duration-200
              ${activeRoute === "tv-shows" ? "bg-secondary/20" : ""}
              focus:bg-secondary/30 focus:scale-110
            `}
          >
            <Ionicons
              name={activeRoute === "tv-shows" ? "tv" : "tv-outline"}
              size={28}
              color={activeRoute === "tv-shows" ? "#CEE3F9" : "#6B7280"}
            />
            <Text
              className={`
                ml-2 text-lg font-semibold
                ${activeRoute === "tv-shows" ? "text-light" : "text-info/60"}
              `}
            >
              TV Shows
            </Text>
          </Pressable>

          {/* Music Button */}
          <Pressable
            focusable={true}
            onPress={() => {
              setActiveRoute("music");
            }}
            className={`
              flex-row items-center px-8 h-[50px]
              rounded-lg overflow-hidden transition-all duration-200
              ${activeRoute === "music" ? "bg-secondary/20" : ""}
              focus:bg-secondary/30 focus:scale-110
            `}
          >
            <Ionicons
              name={
                activeRoute === "music"
                  ? "musical-notes"
                  : "musical-notes-outline"
              }
              size={28}
              color={activeRoute === "music" ? "#CEE3F9" : "#6B7280"}
            />
            <Text
              className={`
                ml-2 text-lg font-semibold
                ${activeRoute === "music" ? "text-light" : "text-info/60"}
              `}
            >
              Music
            </Text>
          </Pressable>

          {/* Other Button */}
          <Pressable
            focusable={true}
            onPress={() => {
              setActiveRoute("other");
            }}
            className={`
              flex-row items-center px-8 h-[50px]
              rounded-lg overflow-hidden transition-all duration-200
              ${activeRoute === "other" ? "bg-secondary/20" : ""}
              focus:bg-secondary/30 focus:scale-110
            `}
          >
            <Ionicons
              name={activeRoute === "other" ? "apps" : "apps-outline"}
              size={28}
              color={activeRoute === "other" ? "#CEE3F9" : "#6B7280"}
            />
            <Text
              className={`
                ml-2 text-lg font-semibold
                ${activeRoute === "other" ? "text-light" : "text-info/60"}
              `}
            >
              Other
            </Text>
          </Pressable>
        </View>

        {/* Settings Button - Moved to right side */}
        <Pressable
          focusable={true}
          onPress={() => {
            setActiveRoute("settings");
          }}
          className={`
            flex-row items-center px-8 h-[50px]
            rounded-lg overflow-hidden transition-all duration-200
            ${activeRoute === "settings" ? "bg-secondary/20" : ""}
            focus:bg-secondary/30 focus:scale-110
          `}
        >
          <Ionicons
            name={activeRoute === "settings" ? "settings" : "settings-outline"}
            size={28}
            color={activeRoute === "settings" ? "#CEE3F9" : "#6B7280"}
          />
          <Text
            className={`
              ml-2 text-lg font-semibold
              ${activeRoute === "settings" ? "text-light" : "text-info/60"}
            `}
          >
            Settings
          </Text>
        </Pressable>
      </View>

      <View className='flex-1'>
        <Slot />
      </View>
    </View>
  );
}
