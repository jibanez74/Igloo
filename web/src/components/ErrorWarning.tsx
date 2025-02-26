import { FiAlertCircle } from "react-icons/fi";

type ErrorWarningProps = {
  error: string;
  isVisible: boolean;
}

export default function ErrorWarning({ error, isVisible }: ErrorWarningProps) {
  return (
    <div
      className='h-10 flex items-center justify-center'
      role='alert'
      aria-live='polite'
    >
      <div
        className={`flex items-center gap-2 text-red-500 text-sm font-medium bg-red-500/10 px-3 py-2 rounded-md transition-all duration-300 ${
          isVisible
            ? "opacity-100 transform scale-100"
            : "opacity-0 transform scale-95"
        } ${error ? "" : "hidden"}`}
      >
        <FiAlertCircle className='h-4 w-4 flex-shrink-0' aria-hidden='true' />
        <span>{error}</span>
      </div>
    </div>
  );
}
