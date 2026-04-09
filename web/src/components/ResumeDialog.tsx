import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { formatTimeSeconds } from "@/lib/format";

type Props = {
  open: boolean;
  savedProgressSec: number | null;
  pending: boolean;
  onResume: () => void;
  onStartFromBeginning: () => void;
};

export default function ResumeDialog({
  open,
  savedProgressSec,
  pending,
  onResume,
  onStartFromBeginning,
}: Props) {
  return (
    <AlertDialog open={open}>
      <AlertDialogContent className="border-slate-700 bg-slate-900">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-white">Resume movie?</AlertDialogTitle>
          <AlertDialogDescription className="text-slate-400">
            {savedProgressSec !== null
              ? `Resume from ${formatTimeSeconds(savedProgressSec)} or start from the beginning.`
              : "Resume your saved progress or start from the beginning."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            onClick={(e) => {
              e.preventDefault();
              onStartFromBeginning();
            }}
            disabled={pending}
            className="border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700 hover:text-white"
          >
            Start from beginning
          </AlertDialogCancel>
          <AlertDialogAction
            variant="accent"
            onClick={(e) => {
              e.preventDefault();
              onResume();
            }}
            disabled={pending}
          >
            Resume
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
