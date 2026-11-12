import type { ReactNode } from "react";
import { FieldHint } from "@/components/ui/field";
import { Label } from "@/components/ui/label";

type PlaybackFieldProps = {
  /** The control's id; the label points at it. */
  htmlFor: string;
  label: string;
  /** The hint's id, for the control's `aria-describedby`. */
  hintId: string;
  hint: string;
  /** The control, then anything that depends on its value. */
  children: ReactNode;
};

/**
 * One field row inside a playback settings card: a single label, its help,
 * then the control (design-system §3.7). `PlaybackSection` is the row for
 * read-only values, which have a heading and no control.
 */
export default function PlaybackField({
  htmlFor,
  label,
  hintId,
  hint,
  children,
}: PlaybackFieldProps) {
  return (
    <div className="grid gap-2 py-5 first:pt-0 last:pb-0">
      <div className="max-w-2xl">
        <Label htmlFor={htmlFor}>{label}</Label>
        <FieldHint id={hintId}>{hint}</FieldHint>
      </div>
      <div className="grid max-w-md gap-2">{children}</div>
    </div>
  );
}
