import {
  ChevronDown,
  ListPlus,
  Share2,
  Download,
  Info,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";

type MovieMoreMenuProps = {
  movieId: number;
};

export default function MovieMoreMenu({ movieId: _movieId }: MovieMoreMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="lg"
          className="min-h-11 rounded-full border-slate-600 bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:border-slate-500 hover:bg-slate-600 focus-visible:ring-amber-400/50"
          aria-label="More options"
        >
          More…
          <ChevronDown className="size-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="min-w-44 border-slate-700 bg-slate-800 text-white"
      >
        <DropdownMenuItem
          className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
          onSelect={(e) => e.preventDefault()}
        >
          <ListPlus className="size-4" aria-hidden="true" />
          Add to list
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
          onSelect={(e) => e.preventDefault()}
        >
          <Share2 className="size-4" aria-hidden="true" />
          Share
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
          onSelect={(e) => e.preventDefault()}
        >
          <Download className="size-4" aria-hidden="true" />
          Download
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer text-slate-200 focus:bg-slate-700 focus:text-white"
          onSelect={(e) => e.preventDefault()}
        >
          <Info className="size-4" aria-hidden="true" />
          Movie info
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
