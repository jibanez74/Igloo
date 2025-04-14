import { View, Text } from "react-native";
import { MaterialCommunityIcons } from "@expo/vector-icons";
import Animated, { FadeIn, FadeOut } from "react-native-reanimated";

type ErrorWarningProps = {
  error: string;
  isVisible: boolean;
};

export default function ErrorWarning({ error, isVisible }: ErrorWarningProps) {
  if (!error || !isVisible) return null;

  return (
    <Animated.View
      entering={FadeIn}
      exiting={FadeOut}
      className="h-10 flex items-center justify-center"
      accessibilityRole="alert"
    >
      <View className="flex flex-row items-center gap-2 bg-red-400/10 px-3 py-2 rounded-md">
        <MaterialCommunityIcons
          name="alert-circle"
          size={16}
          color="#f87171"
          className="flex-shrink-0"
        />
        <Text className="text-red-400 text-sm font-medium">{error}</Text>
      </View>
    </Animated.View>
  );
}
