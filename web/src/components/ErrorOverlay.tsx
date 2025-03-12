type ErrorWarningProps = {
  title?: string;
  message: string;
};

export default function ErrorWarning(props: ErrorWarningProps) {
  return (
    <div class="absolute inset-0 flex items-center justify-center bg-slate-900/90 backdrop-blur-sm text-sky-200 p-8 text-center">
      <div class="max-w-md">
        <p class="text-lg font-medium mb-2">
          {props.title || "Playback Error"}
        </p>
        <p>{props.message}</p>
      </div>
    </div>
  );
}
