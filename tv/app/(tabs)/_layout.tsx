import { Tabs } from "expo-router";
import { useColorScheme } from "@/hooks/useColorScheme";
import { Colors } from "@/constants/Colors";
import { FontAwesome6 } from "@expo/vector-icons";

export default function TabsLayout() {
  const colorScheme = useColorScheme() ?? "light";
  const colors = Colors[colorScheme];

  return (
    <Tabs
      screenOptions={{
        tabBarPosition: "top",
        tabBarActiveTintColor: colors.tabIconSelected,
        tabBarInactiveTintColor: colors.tabIconDefault,
        tabBarStyle: {
          backgroundColor: colors.background,
          borderBottomColor: colors.tint,
          borderBottomWidth: 2,
        },
        tabBarLabelStyle: {
          fontWeight: "bold",
          textTransform: "none",
        },
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: "Home",
          headerShown: false,
          tabBarIcon: ({ color, size }) => (
            <FontAwesome6 name="house" size={size} color={color} />
          ),
        }}
      />

      <Tabs.Screen
        name="movies/index"
        options={{
          title: "Movies",
          headerShown: false,
          tabBarIcon: ({ color, size }) => (
            <FontAwesome6 name="film" size={size} color={color} />
          ),
        }}
      />
    </Tabs>
  );
}
