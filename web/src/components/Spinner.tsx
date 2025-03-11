import { FiLoader } from "solid-icons/fi";

type SpinnerProps = {
  size?: "sm" | "md" | "lg";
  className?: string;
};

export default function Spinner(props: SpinnerProps) {
  const { size = "md", className = "" } = props;

  const sizeClasses = {
    sm: "w-4 h-4",
    md: "w-5 h-5",
    lg: "w-6 h-6",
  };

  return (
    <FiLoader
      class={`text-blue-400 animate-spin ${sizeClasses[size]} ${className}`}
      aria-hidden="true"
    />
  );
}
