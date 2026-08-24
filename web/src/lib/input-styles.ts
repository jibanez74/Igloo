// In dark mode the base `ui/input.tsx` `dark:bg-input/30` wins over
// `bg-slate-50/92`, so the field renders as dark glass — the foreground
// colors need matching `dark:` variants or typed text is near-invisible.
export const lightInputClassName =
  "border-white/25 bg-slate-50/92 text-slate-950 placeholder:text-slate-500 shadow-sm backdrop-blur-sm " +
  "dark:text-slate-50 dark:placeholder:text-slate-400 " +
  "focus-visible:border-ring/70 focus-visible:ring-ring/20";

export const lightInputActionClassName =
  "text-slate-500 hover:text-slate-800 disabled:text-slate-400 " +
  "dark:text-slate-400 dark:hover:text-slate-100 dark:disabled:text-slate-600";

export const inputIconClassName =
  "absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-slate-500 dark:text-slate-400";
