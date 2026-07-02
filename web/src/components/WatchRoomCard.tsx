import { useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Film, Trash2, Users } from "lucide-react";
import {
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import DeleteWatchRoomDialog from "@/components/DeleteWatchRoomDialog";
import { cn } from "@/lib/utils";
import type { WatchRoomType } from "@/types";

type Props = {
  room: WatchRoomType;
};

function getInitials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();
  return `${parts[0].slice(0, 1)}${parts[1].slice(0, 1)}`.toUpperCase();
}

export default function WatchRoomCard({ room }: Props) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const closeRoomButtonRef = useRef<HTMLButtonElement | null>(null);

  const posterUrl = room.movie_poster
    ? buildTmdbImageUrl(room.movie_poster, TMDB_POSTER_SIZE)
    : "";

  // Show up to 4 members (owner + guests); overflow count shown as "+N"
  const MAX_SHOWN = 4;
  const shownMembers = room.members.slice(0, MAX_SHOWN);
  const overflow = room.members.length - MAX_SHOWN;
  const memberNames = room.members.map(member => member.name);
  const displayedMemberNames = memberNames.slice(0, 3).join(", ");
  const remainingMemberCount = memberNames.length - 3;
  const membersSummary =
    memberNames.length <= 3
      ? displayedMemberNames
      : `${displayedMemberNames}, +${remainingMemberCount} more`;

  return (
    <article
      className={cn(
        CARD_INTERACTIVE_SURFACE_CLASS,
        "group flex gap-4 rounded-2xl border border-border bg-card/95 p-4 focus-within:border-ring/40 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background hover:-translate-y-1 hover:border-primary/40 hover:shadow-xl hover:shadow-primary/10",
      )}
      aria-label={`Watch room: ${room.movie_title}`}
    >
      <div className="relative aspect-2/3 w-16 shrink-0 overflow-hidden rounded-lg bg-muted">
        {posterUrl ? (
          <img
            src={posterUrl}
            alt=""
            width={64}
            height={96}
            loading="lazy"
            decoding="async"
            className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
          />
        ) : (
          <div className="flex size-full items-center justify-center">
            <Film className="size-6 text-muted-foreground" aria-hidden="true" />
          </div>
        )}
      </div>

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold text-foreground">
              {room.movie_title}
            </h3>

            <p className="mt-0.5 truncate text-xs text-muted-foreground">
              Hosted by {room.owner.name}
              {room.is_owner && (
                <span className="ml-1 text-primary">(you)</span>
              )}
            </p>
          </div>

          {room.is_owner && (
            <Button
              ref={closeRoomButtonRef}
              variant="ghost"
              size="icon"
              className="size-8 shrink-0 text-muted-foreground hover:text-destructive"
              aria-label={`Close watch room for ${room.movie_title}`}
              onClick={() => setConfirmOpen(true)}
            >
              <Trash2 className="size-4" aria-hidden="true" />
            </Button>
          )}
        </div>

        {room.members.length > 0 && (
          <div className="mt-3 rounded-xl border border-border bg-background/40 p-3">
            <div className="flex items-center gap-2">
              <Users className="size-3.5 text-muted-foreground" aria-hidden="true" />
              <p className="text-xs font-medium text-muted-foreground">
                {room.members.length} member{room.members.length === 1 ? "" : "s"}
              </p>
            </div>

            <div className="mt-2 flex -space-x-2">
              {shownMembers.map(member => (
                <Avatar
                  key={member.id}
                  className="size-7 border-2 border-border"
                >
                  {member.avatar ? (
                    <AvatarImage
                      src={`/api/static/${member.avatar}`}
                      alt={member.name}
                    />
                  ) : null}
                  <AvatarFallback className="bg-accent text-[10px] font-semibold text-foreground">
                    {getInitials(member.name)}
                  </AvatarFallback>
                </Avatar>
              ))}
              {overflow > 0 && (
                <span className="flex size-7 items-center justify-center rounded-full border-2 border-border bg-muted text-[10px] font-semibold text-muted-foreground">
                  +{overflow}
                </span>
              )}
            </div>

            <p className="mt-2 text-xs/5 text-muted-foreground">
              {membersSummary}
            </p>
          </div>
        )}

        <div className="mt-4 flex items-center gap-2">
          <Link
            to="/watch-rooms/$id"
            params={{ id: room.id }}
            className="inline-flex h-9 items-center rounded-md bg-primary px-3 text-xs font-semibold text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background focus-visible:outline-none"
            aria-label={`Join watch room for ${room.movie_title}`}
          >
            Join room
          </Link>
        </div>
      </div>

      {room.is_owner && (
        <DeleteWatchRoomDialog
          roomId={room.id}
          movieTitle={room.movie_title}
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          restoreFocusRef={closeRoomButtonRef}
        />
      )}
    </article>
  );
}
