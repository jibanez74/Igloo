import type { ComponentProps, ReactNode, RefObject } from "react";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { cn } from "@/lib/utils";

type ButtonVariant = ComponentProps<typeof Button>["variant"];

type ConfirmDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  pending?: boolean;
  confirmDisabled?: boolean;
  variant?: ButtonVariant;
  restoreFocusRef?: RefObject<HTMLElement | null>;
  onConfirm: () => void;
  className?: string;
};

export default function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel,
  cancelLabel = "Cancel",
  pending = false,
  confirmDisabled = false,
  variant = "destructive",
  restoreFocusRef,
  onConfirm,
  className,
}: ConfirmDialogProps) {
  const handleOpenChange = (next: boolean) => {
    if (pending && !next) return;
    onOpenChange(next);
  };

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent
        className={cn("border-border bg-card", className)}
        onCloseAutoFocus={
          restoreFocusRef
            ? event => {
                event.preventDefault();
                focusDialogRestoreTarget(restoreFocusRef.current);
              }
            : undefined
        }
      >
        <AlertDialogHeader>
          <AlertDialogTitle
            className={variant === "destructive" ? "text-destructive" : "text-foreground"}
          >
            {title}
          </AlertDialogTitle>
          {description ? (
            <AlertDialogDescription className="text-muted-foreground">
              {description}
            </AlertDialogDescription>
          ) : null}
        </AlertDialogHeader>

        {children}

        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={pending}
            className="border-border bg-muted text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            {cancelLabel}
          </AlertDialogCancel>
          <Button
            type="button"
            variant={variant}
            onClick={onConfirm}
            disabled={pending || confirmDisabled}
            className={variant === "destructive" ? "bg-red-600 text-white hover:bg-red-700" : undefined}
          >
            {pending ? <Spinner className="size-4" aria-hidden="true" /> : null}
            {pending ? `${confirmLabel}...` : confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
