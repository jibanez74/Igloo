import { ActivityIndicator } from "react-native";

type SpinnerProps = {
  size?: "sm" | "md" | "lg";
  color?: string;
};

export default function Spinner({ size = "md", color = "#fbbf24" }: SpinnerProps) {
  const sizeClasses = {
    sm: "w-4 h-4",
    md: "w-5 h-5",
    lg: "w-6 h-6",
  };

  return (
    <ActivityIndicator
      size={size === "sm" ? "small" : size === "lg" ? "large" : "small"}
      color={color}
      className={sizeClasses[size]}
    />
  );
}
