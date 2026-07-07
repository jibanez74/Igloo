import { useId, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
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
import { MonitorSmartphone } from "lucide-react";
import { approveQuickConnect } from "@/lib/api";
import { DEVICES_KEY } from "@/lib/constants";
import { showSuccess, showActionFailed } from "@/lib/toast-helpers";

const CODE_LENGTH = 6;

// The API maps a 404 to a generic message; for this form the 404 always means
// the code itself was wrong or expired, so show something actionable instead.
const INVALID_CODE_MESSAGE =
  "That code is invalid or has expired. Check the code on your device or request a new one.";

export default function QuickConnectApproveCard() {
  const queryClient = useQueryClient();
  const codeId = useId();
  const codeErrorId = `${codeId}-error`;
  const codeDescriptionId = `${codeId}-description`;
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);

  const approveMutation = useMutation({
    mutationFn: (deviceCode: string) => approveQuickConnect(deviceCode),
    onSuccess: res => {
      if (res.error) {
        const message = res.message.startsWith("404")
          ? INVALID_CODE_MESSAGE
          : res.message;
        setError(message);
        showActionFailed("approve device", message);
        return;
      }

      setError(null);
      setCode("");
      showSuccess("Device approved", "It will finish signing in shortly.");

      // The device row is created when the device next polls (a couple of
      // seconds after approval), so refresh the list again shortly after.
      queryClient.invalidateQueries({ queryKey: [DEVICES_KEY] });
      window.setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: [DEVICES_KEY] });
      }, 5000);
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setError(message);
      showActionFailed("approve device", message);
    },
  });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== CODE_LENGTH) {
      setError(`Enter the ${CODE_LENGTH}-character code shown on your device.`);
      return;
    }

    approveMutation.mutate(trimmed);
  };

  return (
    <Card className="border-border/50 bg-muted/30">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-foreground">
          <MonitorSmartphone className="size-5 text-primary" aria-hidden="true" />
          Quick Connect
        </CardTitle>
        <CardDescription className="text-muted-foreground">
          Sign in a TV or mobile app by entering the code it shows you
        </CardDescription>
      </CardHeader>
      <CardContent className="max-w-2xl">
        <form onSubmit={handleSubmit} className="space-y-2" noValidate>
          <Label htmlFor={codeId} className="text-muted-foreground">
            Quick Connect code
          </Label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              id={codeId}
              value={code}
              onChange={e => {
                setCode(e.target.value.toUpperCase());
                setError(null);
              }}
              maxLength={CODE_LENGTH}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              placeholder="e.g. XK4T7P"
              className="font-mono tracking-widest uppercase sm:max-w-48"
              aria-invalid={!!error || undefined}
              aria-describedby={error ? codeErrorId : codeDescriptionId}
            />
            <Button
              type="submit"
              disabled={approveMutation.isPending}
              variant="accent"
              className="w-full sm:w-auto"
            >
              {approveMutation.isPending ? "Approving..." : "Approve device"}
            </Button>
          </div>
          <p id={codeDescriptionId} className="text-xs text-muted-foreground">
            The device gets signed in to your account and appears under Devices
            below.
          </p>
          {error && (
            <p id={codeErrorId} className="text-xs text-destructive" role="alert">
              {error}
            </p>
          )}
        </form>
      </CardContent>
    </Card>
  );
}
