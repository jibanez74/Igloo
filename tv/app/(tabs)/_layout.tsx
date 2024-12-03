import { Tabs } from "expo-router";
import { Ionicons } from "@expo/vector-icons";

export default function TabLayout() {
  return (
    <Tabs
      screenOptions={{
        tabBarPosition: "top",
        headerShown: false,
        tabBarStyle: {
          backgroundColor: "#121F32", // dark
          borderBottomColor: "#1C395E", // primary
          borderBottomWidth: 1,
        },
        tabBarActiveTintColor: "#88BCF4", // secondary
        tabBarInactiveTintColor: "#CEE3F9", // info
      }}
    >
      <Tabs.Screen
        name='index'
        options={{
          title: "Home",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "home" : "home-outline"}
              size={24}
              color={color}
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
              size={24}
              color={color}
            />
          ),
        }}
      />
    </Tabs>
  );
}
