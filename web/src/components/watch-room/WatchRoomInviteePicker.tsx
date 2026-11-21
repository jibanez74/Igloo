import { useDeferredValue, useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Users, X } from "lucide-react";
import { MOTION_MICRO_COLORS_CLASS } from "@/lib/constants";
import { cn, getInitials } from "@/lib/utils";
import { watchRoomInviteUsersQueryOpts } from "@/lib/query-opts";
import { apiErrorMessage } from "@/lib/is-api-failure";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";

type WatchRoomInviteePickerProps = {
  /** Loads the user list only while the dialog holding the picker is open. */
  enabled: boolean;
  selectedUserIds: number[];
  onSelectedUserIdsChange: (ids: number[]) => void;
  description: string;
};

/**
 * The searchable list of people a watch room can invite, with the picked ones
 * as removable chips. Creating a room and editing its invitees share it.
 */
export default function WatchRoomInviteePicker({
  enabled,
  selectedUserIds,
  onSelectedUserIdsChange,
  description,
}: WatchRoomInviteePickerProps) {
  const headingId = useId();
  const searchId = useId();
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);

  const { data: inviteUsersData, isPending: inviteUsersPending } = useQuery(
    watchRoomInviteUsersQueryOpts(enabled),
  );
  const inviteUsers =
    inviteUsersData?.error === false ? inviteUsersData.data.users : [];

  const normalizedSearch = deferredSearch.trim().toLowerCase();
  const filteredUsers =
    normalizedSearch === ""
      ? inviteUsers
      : inviteUsers.filter(user => {
          const haystacks = [user.name, user.email];
          return haystacks.some(value =>
            value.toLowerCase().includes(normalizedSearch),
          );
        });
  const selectedUsers = inviteUsers.filter(user =>
    // selectedUserIds is what the operator checked in this dialog - single digits,
    // hard-bounded by the account roster.
    // react-doctor-disable-next-line react-doctor/js-set-map-lookups
    selectedUserIds.includes(user.id),
  );

  const handleToggleUser = (userId: number, checked: boolean) => {
    if (checked) {
      if (!selectedUserIds.includes(userId)) {
        onSelectedUserIdsChange([...selectedUserIds, userId]);
      }
      return;
    }
    onSelectedUserIdsChange(selectedUserIds.filter(id => id !== userId));
  };

  return (
    <section aria-labelledby={headingId} className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 id={headingId} className="text-sm font-semibold text-foreground">
            Invite people
          </h3>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        <div
          className="inline-flex items-center gap-2 rounded-full border border-border bg-background/40 px-3 py-1 text-xs text-muted-foreground"
          aria-live="polite"
        >
          <Users className="size-3.5 text-muted-foreground" aria-hidden="true" />
          {selectedUserIds.length} selected
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor={searchId} tone="muted">
          Search users
        </Label>
        <Input
          id={searchId}
          value={search}
          onChange={event => setSearch(event.target.value)}
          placeholder="Search by name or email"
          autoComplete="off"
        />
      </div>

      {selectedUsers.length > 0 && (
        <div className="flex flex-wrap gap-2" aria-label="Selected users">
          {selectedUsers.map(user => (
            <Button
              key={user.id}
              type="button"
              variant="outline"
              size="sm"
              shape="pill"
              className="max-w-full"
              onClick={() => handleToggleUser(user.id, false)}
              aria-label={`Remove ${user.name} from invited users`}
            >
              <span className="truncate">{user.name}</span>
              <X className="size-3.5" aria-hidden="true" />
            </Button>
          ))}
        </div>
      )}

      <div className="rounded-lg border border-border bg-background/30">
        {inviteUsersPending ? (
          <div className="flex items-center justify-center gap-2 px-4 py-8 text-sm text-muted-foreground">
            <Spinner aria-hidden="true" />
            Loading users…
          </div>
        ) : inviteUsersData?.error ? (
          <p className="px-4 py-6 text-sm text-destructive">
            {apiErrorMessage(
              inviteUsersData,
              "Couldn’t load users.",
            )}
          </p>
        ) : inviteUsers.length === 0 ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            No other users are available to invite yet.
          </p>
        ) : filteredUsers.length === 0 ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            No users match that search.
          </p>
        ) : (
          <ul className="divide-y divide-border">
            {filteredUsers.map(user => {
              // Same short selectedUserIds list as above.
              // react-doctor-disable-next-line react-doctor/js-set-map-lookups
              const checked = selectedUserIds.includes(user.id);
              return (
                <li key={user.id}>
                  <label
                    className={cn(
                      MOTION_MICRO_COLORS_CLASS,
                      "flex cursor-pointer items-center gap-3 px-4 py-3 hover:bg-muted/50",
                    )}
                  >
                    <Checkbox
                      checked={checked}
                      onCheckedChange={value =>
                        handleToggleUser(user.id, value === true)
                      }
                      aria-label={`Invite ${user.name}`}
                    />
                    <Avatar className="size-9 border border-border">
                      {user.avatar ? (
                        <AvatarImage
                          src={getMediaImageUrl(user.avatar) ?? undefined}
                          alt=""
                        />
                      ) : null}
                      <AvatarFallback className="bg-accent text-xs font-semibold text-foreground">
                        {getInitials(user.name, "?")}
                      </AvatarFallback>
                    </Avatar>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-foreground">
                        {user.name}
                      </span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {user.email}
                      </span>
                    </span>
                  </label>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </section>
  );
}
