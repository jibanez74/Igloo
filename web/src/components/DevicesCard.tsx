import { useId, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Pencil, Smartphone, Tv } from "lucide-react";
import ConfirmDialog from "@/components/ConfirmDialog";
import { renameDevice, revokeDevice } from "@/lib/api";
import { DEVICES_KEY } from "@/lib/constants";
import { devicesQueryOpts } from "@/lib/query-opts";
import { showSuccess, showActionFailed } from "@/lib/toast-helpers";
import type { DeviceType } from "@/types";

function formatDeviceDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleDateString();
}

function devicePlatformLabel(device: DeviceType) {
  const parts = [device.platform, device.app_version].filter(Boolean);
  return parts.join(" · ");
}

export default function DevicesCard() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery(devicesQueryOpts());
  const renameInputId = useId();
  const [deviceToRevoke, setDeviceToRevoke] = useState<DeviceType | null>(null);
  const [deviceToRename, setDeviceToRename] = useState<DeviceType | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const revokeButtonRef = useRef<HTMLButtonElement | null>(null);

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeDevice(id),
    onSuccess: res => {
      if (res.error) {
        showActionFailed("revoke device", res.message);
        return;
      }
      showSuccess("Device revoked");
      queryClient.invalidateQueries({ queryKey: [DEVICES_KEY] });
    },
    onError: () => {
      showActionFailed("revoke device", "An unexpected error occurred");
    },
    onSettled: () => {
      setDeviceToRevoke(null);
    },
  });

  const renameMutation = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) =>
      renameDevice(id, name),
    onSuccess: res => {
      if (res.error) {
        showActionFailed("rename device", res.message);
        return;
      }
      showSuccess("Device renamed");
      setDeviceToRename(null);
      queryClient.invalidateQueries({ queryKey: [DEVICES_KEY] });
    },
    onError: () => {
      showActionFailed("rename device", "An unexpected error occurred");
    },
  });

  const handleRenameSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!deviceToRename) return;

    const name = renameValue.trim();
    if (!name || name === deviceToRename.name) {
      setDeviceToRename(null);
      return;
    }

    renameMutation.mutate({ id: deviceToRename.id, name });
  };

  const devices =
    data?.error === false && data.data?.devices ? data.data.devices : [];

  return (
    <Card className="border-border/50 bg-muted/30">
      <CardHeader>
        <CardTitle asChild className="flex items-center gap-2 text-foreground">
          <h2>
            <Smartphone className="size-5 text-primary" aria-hidden="true" />
            Devices
          </h2>
        </CardTitle>
        <CardDescription className="text-muted-foreground">
          TV and mobile apps signed in to your account
        </CardDescription>
      </CardHeader>
      <CardContent className="max-w-2xl">
        {isLoading ? (
          <p className="text-sm text-muted-foreground">Loading devices...</p>
        ) : data?.error ? (
          <p className="text-sm text-destructive" role="alert">
            {data.message}
          </p>
        ) : devices.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No devices connected. Use Quick Connect or sign in from a device to
            pair it.
          </p>
        ) : (
          <ul className="divide-y divide-border/50">
            {devices.map(device => {
              const isTv = device.platform.toLowerCase().includes("tv");
              const PlatformIcon = isTv ? Tv : Smartphone;
              const isRenaming = deviceToRename?.id === device.id;

              return (
                <li
                  key={device.id}
                  className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <PlatformIcon
                      className="size-5 shrink-0 text-muted-foreground"
                      aria-hidden="true"
                    />
                    <div className="min-w-0">
                      {isRenaming ? (
                        <form
                          onSubmit={handleRenameSubmit}
                          className="flex flex-col gap-2 sm:flex-row sm:items-center"
                        >
                          <Label htmlFor={renameInputId} className="sr-only">
                            New name for {device.name}
                          </Label>
                          <Input
                            id={renameInputId}
                            value={renameValue}
                            onChange={e => setRenameValue(e.target.value)}
                            maxLength={100}
                            autoFocus
                            className="h-8"
                          />
                          <div className="flex gap-2">
                            <Button
                              type="submit"
                              size="sm"
                              variant="accent"
                              disabled={renameMutation.isPending}
                            >
                              {renameMutation.isPending ? "Saving..." : "Save"}
                            </Button>
                            <Button
                              type="button"
                              size="sm"
                              variant="ghost"
                              onClick={() => setDeviceToRename(null)}
                            >
                              Cancel
                            </Button>
                          </div>
                        </form>
                      ) : (
                        <p className="truncate text-sm font-medium text-foreground">
                          {device.name}
                        </p>
                      )}
                      <p className="truncate text-xs text-muted-foreground">
                        {devicePlatformLabel(device)}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        Added {formatDeviceDate(device.created_at)} · Last used{" "}
                        {formatDeviceDate(device.last_used_at)}
                      </p>
                    </div>
                  </div>
                  {!isRenaming && (
                    <div className="flex shrink-0 gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setDeviceToRename(device);
                          setRenameValue(device.name);
                        }}
                        aria-label={`Rename ${device.name}`}
                      >
                        <Pencil className="size-4" aria-hidden="true" />
                        Rename
                      </Button>
                      <Button
                        ref={
                          deviceToRevoke?.id === device.id
                            ? revokeButtonRef
                            : undefined
                        }
                        variant="outline"
                        size="sm"
                        className="border-destructive/50 text-destructive hover:bg-destructive/10 hover:text-destructive"
                        onClick={() => setDeviceToRevoke(device)}
                        aria-label={`Revoke ${device.name}`}
                      >
                        Revoke
                      </Button>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>

      <ConfirmDialog
        open={deviceToRevoke !== null}
        onOpenChange={open => {
          if (!open) setDeviceToRevoke(null);
        }}
        title={`Revoke ${deviceToRevoke?.name ?? "device"}?`}
        description="The device is signed out immediately and must pair again to reconnect."
        confirmLabel="Revoke"
        pending={revokeMutation.isPending}
        restoreFocusRef={revokeButtonRef}
        onConfirm={() => {
          if (deviceToRevoke) {
            revokeMutation.mutate(deviceToRevoke.id);
          }
        }}
      />
    </Card>
  );
}
