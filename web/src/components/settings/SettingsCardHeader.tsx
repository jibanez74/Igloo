import type { LucideIcon } from "lucide-react";
import {
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

type SettingsCardHeaderProps = {
  icon: LucideIcon;
  title: string;
  description?: string;
  /** Optional id for the heading, e.g. to label an aria-live region. */
  titleId?: string;
};

/**
 * The shared Settings card header (design-system §3.7): a glacier lucide icon,
 * an `<h2>` heading rendered through `CardTitle asChild` so card-sectioned
 * pages stay navigable by heading, and an optional muted description.
 */
export default function SettingsCardHeader({
  icon: Icon,
  title,
  description,
  titleId,
}: SettingsCardHeaderProps) {
  return (
    <CardHeader>
      <CardTitle
        asChild
        id={titleId}
        className="flex items-center gap-2 text-foreground"
      >
        <h2>
          <Icon className="size-5 text-primary" aria-hidden="true" />
          {title}
        </h2>
      </CardTitle>
      {description && (
        <CardDescription className="text-muted-foreground">
          {description}
        </CardDescription>
      )}
    </CardHeader>
  );
}
