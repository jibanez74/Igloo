import { Text, TextProps, TextStyle } from "react-native";
import Colors from "@/constants/Colors";

interface ThemedTextProps extends TextProps {
  variant?: "primary" | "secondary" | "info" | "light" | "dark";
  size?: "small" | "medium" | "large" | "xlarge";
  weight?: "normal" | "bold";
}

export default function ThemedText({
  style,
  variant = "light",
  size = "medium",
  weight = "normal",
  ...props
}: ThemedTextProps) {
  const color = {
    primary: Colors.primary,
    secondary: Colors.secondary,
    info: Colors.info,
    light: Colors.light,
    dark: Colors.dark,
  }[variant];

  const fontSize = {
    small: 16,
    medium: 20,
    large: 24,
    xlarge: 32,
  }[size];

  const fontWeight = {
    normal: "400" as const,
    bold: "700" as const,
  }[weight];

  return (
    <Text
      style={[
        {
          color,
          fontSize,
          fontWeight,
        },
        style,
      ]}
      {...props}
    />
  );
}
