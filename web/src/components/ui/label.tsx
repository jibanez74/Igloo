import * as React from "react";

import { cn } from "@/lib/utils";

type LabelProps = React.ComponentProps<"label"> & {
  /** `muted` for a label that sits under a section title of its own. */
  tone?: "default" | "muted";
};

function Label({ className, tone = "default", ...props }: LabelProps) {
  return (
    <label
      data-slot="label"
      className={cn(
        "mb-1 block text-sm font-medium",
        tone === "muted" ? "text-muted-foreground" : "text-foreground",
        "peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
        className
      )}
      {...props}
    />
  );
}

export { Label };

