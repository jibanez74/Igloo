import { View, ViewProps } from "react-native";
import { Colors } from "@/constants/Colors";

interface ThemedViewProps extends ViewProps {
  variant?: "primary" | "secondary" | "dark" | "light";
}

export function ThemedView({
  style,
  variant = "dark",
  ...props
}: ThemedViewProps) {
  const backgroundColor = {
    primary: Colors.primary,
    secondary: Colors.secondary,
    dark: Colors.dark,
    light: Colors.light,
  }[variant];

  return (
    <View
      style={[
        {
          backgroundColor,
        },
        style,
      ]}
      {...props}
    />
  );
}
