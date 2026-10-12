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
import { apiErrorMessage } from "@/lib/is-api-failure";

const CODE_LENGTH = 6;

// The server draws codes from an alphabet without I, L, O, 0 or 1 so they stay
// unambiguous on a TV screen. Anything else can only ever 404, and each lookup
// spends one of the account's ten approval attempts per five minutes.
const CODE_PATTERN = /^[ABCDEFGHJKMNPQRSTUVWXYZ2-9]{6}$/;

// Accepts the code however it arrives — typed, or pasted with the spacing a
// device may use to make it legible. The server drops spaces, tabs and dashes;
// this also drops the non-breaking spaces a rendered code can be copied with,
// which only ever makes a paste succeed that the server would have accepted
// anyway. Every other character is kept so a mistyped one reaches validation
// instead of being silently dropped into a code the user never entered.
function normalizeCode(value: string) {
  return value.replace(/[\s-]/g, "").toUpperCase();
}

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
  const [connectedName, setConnectedName] = useState<string | null>(null);

  // The devices the account had before the approval, so the new one stands
  // out. Null until known: when the approval could not read them first, the
  // first poll supplies them.
  const [knownDeviceIds, setKnownDeviceIds] = useState<number[] | null>(null);

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

  // A pure in-memory lookup, POST only because the code travels in the body.
  // It changes no server state and no query caches it.
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation
  const lookupMutation = useMutation({
    mutationFn: (deviceCode: string) => lookupQuickConnect(deviceCode),
    onSuccess: res => {
      if (res.error) {
        setError(res.status === 404 ? INVALID_CODE_MESSAGE : apiErrorMessage(res, "Couldn’t check that code. Please try again."));
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

  // The mutationFn refreshes [DEVICES_KEY] itself via fetchQuery, and the poll
  // below keeps it current, so there is nothing left to invalidate.
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation
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
          setError(apiErrorMessage(res, "Couldn’t approve that device. Please try again."));
        }
        showActionFailed("approve device", res);
        return;
      }

      setError(null);
      setKnownDeviceIds(baselineDeviceIds);
      setStep("waiting");
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setError(message);
      showActionFailed("approve device", message);
    },
  });

  // While waiting, poll the shared devices query; DevicesCard observes the
  // same key, so its list updates live as soon as the device finishes.
  const waiting = step === "waiting";
  const { data: devicesData } = useQuery({
    ...devicesQueryOpts(),
    enabled: waiting,
    staleTime: 0,
    refetchInterval: waiting ? 2000 : false,
  });
  const polledDevices =
    waiting && devicesData?.error === false ? devicesData.data.devices : null;

  // State adjusted while rendering rather than in an effect: the first good
  // poll supplies the baseline when the approval could not, and a matching
  // device absent from it is the one that just signed in.
  if (polledDevices && knownDeviceIds === null) {
    setKnownDeviceIds(polledDevices.map(device => device.id));
  }
  const connectedDevice =
    polledDevices && knownDeviceIds
      ? polledDevices.find(
          device =>
            // One user's own devices: a handful of entries.
            // react-doctor-disable-next-line react-doctor/js-set-map-lookups
            !knownDeviceIds.includes(device.id) &&
            matchesPendingDevice(device, pendingDevice),
        )
      : undefined;
  if (connectedDevice) {
    setConnectedName(connectedDevice.name);
    setStep("done");
  }

  useEffect(() => {
    if (step === "done" && connectedName) {
      showSuccess("Device connected", `${connectedName} is now signed in.`);
    }
  }, [step, connectedName]);

  // Stop waiting after the deadline whatever the polls return, failures
  // included. The device can still finish later, so the devices list is
  // marked stale rather than left showing the rows from before the approval.
  useEffect(() => {
    if (!waiting) return;

    const timeout = window.setTimeout(() => {
      setConnectedName(null);
      setStep("done");
      void queryClient.invalidateQueries({
        queryKey: devicesQueryOpts().queryKey,
      });
    }, WAIT_FOR_DEVICE_MS);

    return () => window.clearTimeout(timeout);
  }, [waiting, queryClient]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (code.length !== CODE_LENGTH) {
      setError(`Enter the ${CODE_LENGTH}-character code shown on your device.`);
      return;
    }

    if (!CODE_PATTERN.test(code)) {
      setError(
        "Codes never contain I, L, O, 0 or 1. Check the code on your device.",
      );
      return;
    }

    lookupMutation.mutate(code);
  };

  const handleBack = () => {
    setPendingDevice(null);
    setError(null);
    setStep("enter");
  };

  const handleReset = () => {
    setCode("");
    setPendingDevice(null);
    setKnownDeviceIds(null);
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
                  setCode(normalizeCode(e.target.value));
                  setError(null);
                }}
                autoComplete="off"
                autoCapitalize="characters"
                spellCheck={false}
                placeholder="e.g. XK4T7P"
                className="font-mono tracking-widest uppercase placeholder:tracking-normal placeholder:normal-case sm:max-w-48"
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
              {CODE_LENGTH} characters, never containing I, L, O, 0 or 1.
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
                onClick={() => approveMutation.mutate(code)}
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
