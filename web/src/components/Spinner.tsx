import { FiLoader } from "react-icons/fi";

interface SpinnerProps {
  size?: "sm" | "md" | "lg";
  className?: string;
}

export default function Spinner({ size = "md", className = "" }: SpinnerProps) {
  const sizeClasses = {
    sm: "w-4 h-4",
    md: "w-5 h-5",
    lg: "w-6 h-6",
  };

  return (
    <FiLoader
      className={`text-blue-400 animate-spin ${sizeClasses[size]} ${className}`}
      aria-hidden='true'
    />
  );
}
