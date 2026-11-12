import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import {
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

type SettingsCardHeaderProps = {
  icon: LucideIcon;
  title: string;
  description?: string;
  /** Optional id for the heading, e.g. to label an aria-live region. */
  titleId?: string;
  /** `destructive` for the Danger zone: title, icon and description in red. */
  tone?: "default" | "destructive";
  /** The card's own action (Add user), at the end of the header's row. */
  action?: ReactNode;
};

/**
 * The shared Settings card header (design-system §3.7): a glacier lucide icon,
 * an `<h2>` heading rendered through `CardTitle asChild` so card-sectioned
 * pages stay navigable by heading, and an optional muted description. With an
 * `action` the two share a row from `sm` and stack on a phone.
 */
export default function SettingsCardHeader({
  icon: Icon,
  title,
  description,
  titleId,
  tone = "default",
  action,
}: SettingsCardHeaderProps) {
  const destructive = tone === "destructive";

  const heading = (
    <>
      <CardTitle
        asChild
        id={titleId}
        tone={tone}
        className="flex items-center gap-2"
      >
        <h2>
          <Icon
            className={cn("size-5", !destructive && "text-primary")}
            aria-hidden="true"
          />
          {title}
        </h2>
      </CardTitle>
      {description && (
        <CardDescription className={cn(destructive && "text-destructive")}>
          {description}
        </CardDescription>
      )}
    </>
  );

  if (!action) {
    return <CardHeader>{heading}</CardHeader>;
  }

  return (
    <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="grid gap-2">{heading}</div>
      {action}
    </CardHeader>
  );
}
