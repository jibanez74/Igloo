import PlaylistFormDialog from "./PlaylistFormDialog";
import type { NullableString } from "@/types";

type PlaylistData = {
  id: number;
  name: string;
  description: NullableString;
  is_public: boolean;
};

type EditPlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  playlist: PlaylistData;
};

export default function EditPlaylistDialog({
  open,
  onOpenChange,
  playlist,
}: EditPlaylistDialogProps) {
  return (
    <PlaylistFormDialog
      mode="edit"
      open={open}
      onOpenChange={onOpenChange}
      playlist={playlist}
    />
  );
}
