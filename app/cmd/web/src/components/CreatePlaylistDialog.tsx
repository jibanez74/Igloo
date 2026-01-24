import PlaylistFormDialog from "./PlaylistFormDialog";

type CreatePlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export default function CreatePlaylistDialog({
  open,
  onOpenChange,
}: CreatePlaylistDialogProps) {
  return (
    <PlaylistFormDialog mode="create" open={open} onOpenChange={onOpenChange} />
  );
}
