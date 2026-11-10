import { Info, ListPlus, Pencil, Radio, Trash2 } from "lucide-react";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  SETTINGS_SELECT_CONTENT_CLASS,
} from "@/lib/constants";

// Each stage renders one overlay already open, built the way production call
// sites build it today (their className overrides included), so the baseline
// is what readers see and a later variant has to reproduce it exactly.

const noop = () => {};

const STREAM_PROFILES = [
  ["recommended", "Use recommended"],
  ["2160", "2160p · 16 Mbps"],
  ["1080", "1080p · 8 Mbps"],
  ["720", "720p · 3 Mbps"],
] as const;

function DialogStage() {
  return (
    <Dialog open onOpenChange={noop}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add User</DialogTitle>
          <DialogDescription>
            Create a new user account.
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={event => event.preventDefault()}>
          <div className="space-y-2">
            <Label htmlFor="gallery-dialog-name">
              Name
            </Label>
            <Input id="gallery-dialog-name" placeholder="Full name" />
          </div>
          <DialogFooter className="pt-2">
            <Button type="button" variant="outline">
              Cancel
            </Button>
            <Button type="submit" variant="accent">
              Create User
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function AlertDialogStage() {
  return (
    <ConfirmDialog
      open
      onOpenChange={noop}
      title="Delete playlist"
      description="Are you sure you want to delete “Weekend Marathon”? This action cannot be undone."
      confirmLabel="Delete"
      onConfirm={noop}
    />
  );
}

function SelectStage() {
  return (
    <div className="w-80">
      <Label htmlFor="gallery-open-select">Profile</Label>
      <Select open value="recommended" onValueChange={noop}>
        <SelectTrigger
          id="gallery-open-select"
          variant="inset"
          size="lg"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
          {STREAM_PROFILES.map(([value, label]) => (
            <SelectItem
              key={value}
              value={value}
            >
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function DropdownStage() {
  return (
    <DropdownMenu open onOpenChange={noop}>
      <DropdownMenuTrigger asChild>
        <Button variant="outline">More options</Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="border-border bg-muted"
      >
        <DropdownMenuItem>
          <Radio aria-hidden="true" />
          Watch Together
        </DropdownMenuItem>
        <DropdownMenuItem>
          <ListPlus aria-hidden="true" />
          Add to Playlist
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Pencil aria-hidden="true" />
          Edit
        </DropdownMenuItem>
        <DropdownMenuItem disabled>
          <Info aria-hidden="true" />
          Technical Details
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive">
          <Trash2 aria-hidden="true" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function PopoverStage() {
  return (
    <Popover open onOpenChange={noop}>
      <PopoverTrigger asChild>
        <Button variant="outline">Notifications</Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-80 border-border bg-card p-0"
      >
        <p className="border-b border-border px-4 py-3 text-sm font-semibold text-foreground">
          Notifications
        </p>
        <p className="px-4 py-3 text-sm text-muted-foreground">
          Could we add Dune: Part Two to the library?
        </p>
      </PopoverContent>
    </Popover>
  );
}

const renderStage = (name: string) => {
  switch (name) {
    case "dialog":
      return <DialogStage />;
    case "alert-dialog":
      return <AlertDialogStage />;
    case "select":
      return <SelectStage />;
    case "dropdown":
      return <DropdownStage />;
    case "popover":
      return <PopoverStage />;
    default:
      return null;
  }
};

type GalleryOverlayStageProps = {
  name: string;
};

/** One open overlay on an otherwise empty page; an unknown name shows none. */
export default function GalleryOverlayStage({
  name,
}: GalleryOverlayStageProps) {
  const stage = renderStage(name);

  return (
    <div
      data-gallery-overlay={stage ? name : "unknown"}
      className="flex min-h-svh items-start justify-center px-6 pt-24"
    >
      {stage ?? (
        <p className="text-sm text-muted-foreground">
          Unknown overlay “{name}”.
        </p>
      )}
    </div>
  );
}
