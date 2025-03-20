import { FiLoader } from "solid-icons/fi";

type SpinnerProps = {
  size?: "sm" | "md" | "lg";
};

export default function Spinner(props: SpinnerProps) {
  const { size = "md" } = props;

  const sizeClasses = {
    sm: "w-4 h-4",
    md: "w-5 h-5",
    lg: "w-6 h-6",
  };

  return (
    <FiLoader
      class={`text-yellow-300 animate-spin ${sizeClasses[size]}`}
      aria-hidden="true"
    />
  );
}
