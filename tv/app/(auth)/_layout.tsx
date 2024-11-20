import { useEffect } from "react";
import { View } from "react-native";
import { Tabs } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { useColorScheme } from "react-native";
import { useAuth } from "@/context/AuthContext";
import { router } from "expo-router";

export default function AuthLayout() {
  const { user } = useAuth();
  const colorScheme = useColorScheme();

  useEffect(() => {
    if (!user) {
      router.replace("/login");
    }
  }, [user]);

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarPosition: "top",
        tabBarStyle: {
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          height: 90,
          backgroundColor: "transparent",
          borderBottomWidth: 0,
          elevation: 0,
          paddingHorizontal: 32,
        },
        tabBarBackground: () => (
          <View className='absolute inset-0 bg-gradient-to-b from-dark via-dark/80 to-transparent' />
        ),
        tabBarActiveTintColor: "#CEE3F9",
        tabBarInactiveTintColor: "#6B7280",
        tabBarLabelStyle: {
          fontSize: 18,
          fontWeight: "600",
          marginLeft: 12,
        },
        tabBarItemStyle: {
          height: 90,
          padding: 0,
          margin: 0,
        },
        // TV-optimized tab button
        tabBarButton: props => (
          <View
            focusable={true}
            className={`
              flex-row items-center justify-center px-8 h-full
              rounded-lg overflow-hidden transition-all duration-200
              hover:bg-primary/30
              focus:bg-secondary/20 focus:scale-110
              active:bg-secondary/30
            `}
          >
            {props.children}
          </View>
        ),
      }}
    >
      <Tabs.Screen
        name='index'
        options={{
          title: "Home",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "home" : "home-outline"}
              size={32}
              color={color}
              className='transition-all duration-200'
            />
          ),
        }}
      />
      <Tabs.Screen
        name='movies'
        options={{
          title: "Movies",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "film" : "film-outline"}
              size={32}
              color={color}
              className='transition-all duration-200'
            />
          ),
        }}
      />
      <Tabs.Screen
        name='tv-shows'
        options={{
          title: "TV Shows",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "tv" : "tv-outline"}
              size={32}
              color={color}
              className='transition-all duration-200'
            />
          ),
        }}
      />
      <Tabs.Screen
        name='profile'
        options={{
          title: "Profile",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "person" : "person-outline"}
              size={32}
              color={color}
              className='transition-all duration-200'
            />
          ),
        }}
      />
    </Tabs>
  );
}
