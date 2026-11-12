import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// One column for every Settings tab: the same width and the same gap between
// cards. The scroll margin is for a form that ends in a sticky Save bar: the
// bar overlays the bottom of the viewport, so the browser's scroll to a
// focused control must stop above it (the bar plus its bottom offset, tallest
// on a phone with the mini player showing). It costs nothing on a tab
// without a bar.
const PAGE_CLASS =
  "@container max-w-5xl space-y-6 [&_:is(button,input,select)]:scroll-mb-72 sm:[&_:is(button,input,select)]:scroll-mb-44";

type SettingsPageProps =
  | ({ as?: "div" } & ComponentProps<"div">)
  | ({ as: "form" } & ComponentProps<"form">);

/**
 * The body of a Settings tab (design-system §3.7). `as="form"` is for a tab
 * that is one form saved by a Save bar (General, Libraries).
 */
export default function SettingsPage(props: SettingsPageProps) {
  if (props.as === "form") {
    const { as, className, ...rest } = props;
    void as;
    return <form className={cn(PAGE_CLASS, className)} {...rest} />;
  }

  const { as, className, ...rest } = props;
  void as;
  return <div className={cn(PAGE_CLASS, className)} {...rest} />;
}
