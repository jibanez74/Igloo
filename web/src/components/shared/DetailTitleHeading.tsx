import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type DetailTitleHeadingProps = {
  /** The page's skip-link target and `aria-labelledby` anchor. */
  id: string;
  title: string;
  year: number | null;
  /** Machine-readable date behind the year, when the catalog has one. */
  dateTime: string | null;
};

export default function DetailTitleHeading({
  id,
  title,
  year,
  dateTime,
}: DetailTitleHeadingProps) {
  return (
    <h1
      id={id}
      tabIndex={-1}
      className={cn(
        "flex w-full max-w-full min-w-0 flex-col gap-1 rounded-sm text-2xl font-bold wrap-break-word text-white drop-shadow-lg sm:gap-0 sm:text-3xl lg:flex-row lg:flex-wrap lg:items-baseline lg:gap-x-3 lg:text-4xl xl:text-5xl",
        FOCUS_VISIBLE_RING_CLASS,
      )}
    >
      <span className="min-w-0">{title}</span>
      {year != null && (
        <span className="shrink-0 font-normal text-white/80 sm:text-3xl lg:text-4xl xl:text-5xl">
          (<time dateTime={dateTime ?? undefined}>{year}</time>)
        </span>
      )}
    </h1>
  );
}
