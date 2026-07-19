import { useEffect, useId, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { MonitorSmartphone } from "lucide-react";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import { approveQuickConnect, lookupQuickConnect } from "@/lib/api";
import { SETTINGS_CARD_SURFACE_CLASS } from "@/lib/constants";
import { devicesQueryOpts } from "@/lib/query-opts";
import { showSuccess, showActionFailed } from "@/lib/toast-helpers";
import type {
  ApiResponseType,
  DeviceType,
  DevicesListResponseType,
  QuickConnectLookupType,
} from "@/types";

const CODE_LENGTH = 6;

// After approval the device row only appears once the device polls again;
// stop watching for it after this long and show a softer confirmation.
const WAIT_FOR_DEVICE_MS = 30_000;

// The API maps a 404 to a generic message; for this form the 404 always means
// the code itself was wrong or expired, so show something actionable instead.
const INVALID_CODE_MESSAGE =
  "That code is invalid or has expired. Check the code on your device or request a new one.";

type Step = "enter" | "confirm" | "waiting" | "done";

type ApprovalResult = {
  baselineDeviceIds: number[] | null;
  response: Awaited<ReturnType<typeof approveQuickConnect>>;
};

function getDeviceIds(
  response: ApiResponseType<DevicesListResponseType>,
): number[] | null {
  if (response.error || !response.data?.devices) return null;

  return response.data.devices.map(device => device.id);
}

function matchesPendingDevice(
  device: DeviceType,
  pendingDevice: QuickConnectLookupType | null,
) {
  if (!pendingDevice) return false;

  return (
    device.name === pendingDevice.device_name &&
    device.platform === pendingDevice.platform &&
    device.app_version === pendingDevice.app_version
  );
}

export default function QuickConnectApproveCard() {
  const queryClient = useQueryClient();
  const codeId = useId();
  const codeErrorId = `${codeId}-error`;
  const codeDescriptionId = `${codeId}-description`;
  const [step, setStep] = useState<Step>("enter");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pendingDevice, setPendingDevice] =
    useState<QuickConnectLookupType | null>(null);
  const [knownDeviceIds, setKnownDeviceIds] = useState<number[] | null>(null);
  const [waitDeadline, setWaitDeadline] = useState<number | null>(null);
  const [connectedName, setConnectedName] = useState<string | null>(null);

  const codeInputRef = useRef<HTMLInputElement | null>(null);
  const stepHeadingRef = useRef<HTMLHeadingElement | null>(null);
  const prevStepRef = useRef<Step>("enter");

  // Keep keyboard and screen-reader users oriented as the card swaps its
  // content between steps.
  useEffect(() => {
    if (prevStepRef.current === step) return;
    prevStepRef.current = step;

    if (step === "enter") {
      codeInputRef.current?.focus();
    } else {
      stepHeadingRef.current?.focus();
    }
  }, [step]);

  const lookupMutation = useMutation({
    mutationFn: (deviceCode: string) => lookupQuickConnect(deviceCode),
    onSuccess: res => {
      if (res.error) {
        setError(res.status === 404 ? INVALID_CODE_MESSAGE : res.message);
        return;
      }

      setError(null);
      setPendingDevice(res.data);
      setStep("confirm");
    },
    onError: () => {
      setError("An unexpected error occurred");
    },
  });

  const approveMutation = useMutation({
    mutationFn: async (deviceCode: string): Promise<ApprovalResult> => {
      let baselineDeviceIds: number[] | null = null;

      try {
        baselineDeviceIds = getDeviceIds(
          await queryClient.fetchQuery({
            ...devicesQueryOpts(),
            staleTime: 0,
          }),
        );
      } catch {
        baselineDeviceIds = null;
      }

      return {
        baselineDeviceIds,
        response: await approveQuickConnect(deviceCode),
      };
    },
    onSuccess: ({ baselineDeviceIds, response: res }) => {
      if (res.error) {
        // A 404 here means the code expired (or was raced) between the
        // lookup and the approval, so start over from the code input.
        if (res.status === 404) {
          setError(INVALID_CODE_MESSAGE);
          setStep("enter");
        } else {
          setError(res.message);
        }
        showActionFailed("approve device", res.message);
        return;
      }

      setError(null);
      setKnownDeviceIds(baselineDeviceIds);
      setWaitDeadline(Date.now() + WAIT_FOR_DEVICE_MS);
      setStep("waiting");
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setError(message);
      showActionFailed("approve device", message);
    },
  });

  // While waiting, poll the shared devices query; DevicesCard observes the
  // same key, so its list updates live as soon as the device finishes. The
  // interval callback runs after every fetch, which makes it the natural
  // place to spot the new device (or give up at the deadline) and stop.
  useQuery({
    ...devicesQueryOpts(),
    enabled: step === "waiting",
    staleTime: 0,
    refetchInterval: query => {
      if (step !== "waiting") return false;

      const data = query.state.data;
      const devices =
        data && data.error === false && data.data?.devices
          ? data.data.devices
          : null;
      if (!devices) {
        return 2000;
      }

      const currentKnownDeviceIds = knownDeviceIds;
      if (currentKnownDeviceIds === null) {
        setKnownDeviceIds(devices.map(device => device.id));
        return 2000;
      }

      const fresh = devices.find(
        device =>
          !currentKnownDeviceIds.includes(device.id) &&
          matchesPendingDevice(device, pendingDevice),
      );
      if (fresh) {
        setConnectedName(fresh.name);
        setStep("done");
        showSuccess("Device connected", `${fresh.name} is now signed in.`);
        return false;
      }

      if (waitDeadline !== null && Date.now() > waitDeadline) {
        setConnectedName(null);
        setStep("done");
        return false;
      }

      return 2000;
    },
  });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== CODE_LENGTH) {
      setError(`Enter the ${CODE_LENGTH}-character code shown on your device.`);
      return;
    }

    lookupMutation.mutate(trimmed);
  };

  const handleBack = () => {
    setPendingDevice(null);
    setError(null);
    setStep("enter");
  };

  const handleReset = () => {
    setCode("");
    setPendingDevice(null);
    setKnownDeviceIds([]);
    setWaitDeadline(null);
    setConnectedName(null);
    setError(null);
    setStep("enter");
  };

  return (
    <Card className={SETTINGS_CARD_SURFACE_CLASS}>
      <SettingsCardHeader
        icon={MonitorSmartphone}
        title="Quick Connect"
        description="Sign in a TV or mobile app by entering the code it shows you"
      />
      <CardContent className="max-w-2xl">
        {step === "enter" && (
          <form onSubmit={handleSubmit} className="space-y-2" noValidate>
            <Label htmlFor={codeId} className="text-muted-foreground">
              Quick Connect code
            </Label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                ref={codeInputRef}
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
                disabled={lookupMutation.isPending}
                variant="accent"
                className="w-full sm:w-auto"
              >
                {lookupMutation.isPending ? "Checking..." : "Continue"}
              </Button>
            </div>
            <p id={codeDescriptionId} className="text-xs text-muted-foreground">
              You&apos;ll see which device is asking before it gets signed in.
            </p>
            {error && (
              <p
                id={codeErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {error}
              </p>
            )}
          </form>
        )}

        {step === "confirm" && pendingDevice && (
          <div className="space-y-3">
            <h3
              ref={stepHeadingRef}
              tabIndex={-1}
              className="text-sm font-medium text-foreground outline-hidden"
            >
              Approve this device?
            </h3>
            <dl className="space-y-1 text-sm">
              <div className="flex gap-2">
                <dt className="text-muted-foreground">Name</dt>
                <dd className="font-medium text-foreground">
                  {pendingDevice.device_name}
                </dd>
              </div>
              {pendingDevice.platform && (
                <div className="flex gap-2">
                  <dt className="text-muted-foreground">Platform</dt>
                  <dd className="text-foreground">{pendingDevice.platform}</dd>
                </div>
              )}
              {pendingDevice.app_version && (
                <div className="flex gap-2">
                  <dt className="text-muted-foreground">App version</dt>
                  <dd className="text-foreground">
                    {pendingDevice.app_version}
                  </dd>
                </div>
              )}
            </dl>
            <p className="text-xs text-muted-foreground">
              The device gets signed in to your account and appears under
              Devices below.
            </p>
            {error && (
              <p className="text-xs text-destructive" role="alert">
                {error}
              </p>
            )}
            <div className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="accent"
                disabled={approveMutation.isPending}
                onClick={() => approveMutation.mutate(code.trim().toUpperCase())}
                className="w-full sm:w-auto"
              >
                {approveMutation.isPending ? "Approving..." : "Approve device"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={handleBack}
                className="w-full sm:w-auto"
              >
                Back
              </Button>
            </div>
          </div>
        )}

        {step === "waiting" && (
          <div role="status" className="space-y-2">
            <h3
              ref={stepHeadingRef}
              tabIndex={-1}
              className="flex items-center gap-2 text-sm font-medium text-foreground outline-hidden"
            >
              <Spinner className="size-4 text-primary" aria-hidden="true" />
              Waiting for {pendingDevice?.device_name ?? "the device"} to
              finish signing in...
            </h3>
            <p className="text-xs text-muted-foreground">
              Approved. The device picks up its sign-in the next time it
              checks, usually within a few seconds.
            </p>
          </div>
        )}

        {step === "done" && (
          <div role="status" className="space-y-3">
            <h3
              ref={stepHeadingRef}
              tabIndex={-1}
              className="text-sm font-medium text-foreground outline-hidden"
            >
              {connectedName
                ? `${connectedName} is connected`
                : "Device approved"}
            </h3>
            <p className="text-xs text-muted-foreground">
              {connectedName
                ? "It now appears under Devices below."
                : "The device should appear under Devices shortly."}
            </p>
            <Button
              type="button"
              variant="outline"
              onClick={handleReset}
              className="w-full sm:w-auto"
            >
              Pair another device
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
