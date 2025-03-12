import { FiAlertCircle } from "solid-icons/fi";

type ErrorWarningProps = {
  error: string;
  isVisible: boolean;
}

export default function ErrorWarning(props: ErrorWarningProps) {
  return (
    <div
      class='h-10 flex items-center justify-center'
      role='alert'
      aria-live='polite'
    >
      <div
        class={`flex items-center gap-2 text-red-500 text-sm font-medium bg-red-500/10 px-3 py-2 rounded-md transition-all duration-300 ${
          props.isVisible
            ? "opacity-100 transform scale-100"
            : "opacity-0 transform scale-95"
        } ${props.error ? "" : "hidden"}`}
      >
        <FiAlertCircle class='h-4 w-4 flex-shrink-0' aria-hidden='true' />
        <span>{props.error}</span>
      </div>
    </div>
  );
}
