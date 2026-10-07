import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent, ReactNode } from "react";
import { Apple, CircuitBoard, Cpu, MonitorCog, Server } from "lucide-react";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import DevicePlaybackCards from "@/components/settings/DevicePlaybackCards";
import PlaybackSection from "@/components/settings/PlaybackSection";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import TrailerPreferencesCard from "@/components/settings/TrailerPreferencesCard";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import {
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
  SETTINGS_SELECT_CONTENT_CLASS,
  SETTINGS_SELECT_ITEM_CLASS,
  SETTINGS_SELECT_TRIGGER_CLASS,
} from "@/lib/constants";
import { updatePlaybackSettings } from "@/lib/api";
import { parseMbpsInput } from "@/lib/playback";
import {
  authUserFrom,
  authUserQueryOpts,
  playbackSettingsFromResponse,
  playbackSettingsQueryOpts,
} from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  HardwareAccelerationDevice,
  PlaybackSettingsResponseType,
  PlaybackSettingsType,
  UpdatePlaybackSettingsRequest,
} from "@/types";
import { routeHead } from "@/lib/route-head";

const PLAYBACK_SETTINGS_HEAD = routeHead("Playback Settings");

export const Route = createFileRoute("/_auth/settings/playback")({
  head: () => PLAYBACK_SETTINGS_HEAD,
  component: PlaybackSettings,
});

const SERVER_UPLOAD_MAX_MBPS = 100_000;
const SERVER_UPLOAD_VALIDATION_MESSAGE =
  `Server upload bandwidth must be greater than 0 and less than ${SERVER_UPLOAD_MAX_MBPS} Mbps.`;

type HardwareOption = {
  value: HardwareAccelerationDevice;
  label: string;
  description: string;
  icon: ReactNode;
};

const HARDWARE_OPTIONS: HardwareOption[] = [
  {
    value: "cpu",
    label: "CPU",
    description: "Use software encoding on the host CPU.",
    icon: <Cpu className="size-4 text-muted-foreground" aria-hidden="true" />,
  },
  {
    value: "apple",
    label: "Apple VideoToolbox",
    description: "Use Apple hardware acceleration on supported Macs.",
    icon: <Apple className="size-4 text-muted-foreground" aria-hidden="true" />,
  },
  {
    value: "nvidia",
    label: "NVIDIA NVENC",
    description: "Use NVIDIA GPU acceleration when available.",
    icon: <CircuitBoard className="size-4 text-muted-foreground" aria-hidden="true" />,
  },
  {
    value: "intel",
    label: "Intel Quick Sync",
    description: "Use Intel GPU acceleration when available.",
    icon: <MonitorCog className="size-4 text-muted-foreground" aria-hidden="true" />,
  },
];

function isHardwareAccelerationDevice(
  value: string,
): value is HardwareAccelerationDevice {
  return HARDWARE_OPTIONS.some(option => option.value === value);
}

function isServerUploadOutOfRange(form: UpdatePlaybackSettingsRequest) {
  return (
    form.server_upload_mbps != null &&
    (form.server_upload_mbps <= 0 ||
      form.server_upload_mbps >= SERVER_UPLOAD_MAX_MBPS)
  );
}

function PlaybackSettings() {
  const { data: authData, isLoading: authLoading } = useQuery(
    authUserQueryOpts(),
  );
  const user = authUserFrom(authData);
  const { data, isLoading } = useQuery(playbackSettingsQueryOpts());

  const settings = playbackSettingsFromResponse(data);

  if (authLoading || isLoading) {
    return <SettingsLoadingCard label="Loading playback settings..." />;
  }

  if (authData?.error || user === null) {
    return (
      <SettingsErrorCard
        title="Settings unavailable"
        message={
          authData?.error
            ? authData.message || "Failed to load user information."
            : "User information not available."
        }
      />
    );
  }

  if (data?.error) {
    return (
      <SettingsErrorCard
        title="Settings unavailable"
        message={data.message || "Failed to load playback settings."}
      />
    );
  }

  if (!settings) {
    return null;
  }

  return (
    <div className="max-w-5xl space-y-6">
      <DevicePlaybackCards
        settings={settings}
        userId={user.id}
        isAdmin={user.is_admin}
      />
      <TrailerPreferencesCard />
      {user.is_admin && <ServerPlaybackForm settings={settings} />}
    </div>
  );
}

type ServerPlaybackFormProps = {
  settings: PlaybackSettingsType;
};

function hardwareOption(device: HardwareAccelerationDevice | undefined) {
  return HARDWARE_OPTIONS.find(option => option.value === device);
}

function hardwareOptionLabel(device: HardwareAccelerationDevice) {
  return hardwareOption(device)?.label ?? device;
}

/**
 * What new transcodes actually run on, from the saved settings rather than
 * the form: the env value only seeds a fresh database, and the startup probe
 * can refuse the stored device, so the dropdown alone can mislead. A stored
 * device the server runs is confirmed; one it refused is a standing
 * condition, so the notice is destructive for as long as it lasts (§3.7).
 * A cap below the catalog is named because it changes which modes a 4K file
 * offers; the server decides which devices are capped.
 */
function EffectiveDeviceNotice({ settings }: { settings: PlaybackSettingsType }) {
  const effective = settings.effective_hardware_acceleration_device;
  const refused = effective !== settings.hardware_acceleration_device;
  const capped = settings.profiles.some(
    profile => profile.height > settings.max_transcode_height,
  );
  const cappedNote = capped
    ? ` Transcodes are capped at ${settings.max_transcode_height}p.`
    : "";

  if (refused) {
    return (
      <p className="text-sm text-destructive">
        {hardwareOptionLabel(settings.hardware_acceleration_device)} is not
        available on this server ({settings.hardware_fallback_reason}), so
        transcodes run on the {hardwareOptionLabel(effective)}.
        {cappedNote}
      </p>
    );
  }

  return (
    <p className="text-sm text-muted-foreground">
      In use for new transcodes: {hardwareOptionLabel(effective)}.
      {cappedNote}
    </p>
  );
}

type PlaybackSettingsQueryData = {
  error: false;
  message?: string;
  data?: PlaybackSettingsResponseType;
};

/** The server-wide half of the page: admin-only, and the only part that saves. */
function formFromSettings(
  settings: PlaybackSettingsType,
): UpdatePlaybackSettingsRequest {
  return {
    server_upload_mbps: settings.server_upload_mbps,
    hardware_acceleration_device: settings.hardware_acceleration_device,
  };
}

function formMatchesSettings(
  form: UpdatePlaybackSettingsRequest,
  settings: PlaybackSettingsType,
) {
  const saved = formFromSettings(settings);
  return (
    form.server_upload_mbps === saved.server_upload_mbps &&
    form.hardware_acceleration_device === saved.hardware_acceleration_device
  );
}

function ServerPlaybackForm({ settings }: ServerPlaybackFormProps) {
  const queryClient = useQueryClient();
  const serverUploadMbpsId = useId();
  const hardwareDeviceId = useId();
  const statusId = useId();

  const [form, setForm] = useState<UpdatePlaybackSettingsRequest>(() =>
    formFromSettings(settings),
  );
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [validationMessage, setValidationMessage] = useState("");
  const [, startTransition] = useTransition();
  const isDirty = !formMatchesSettings(form, syncedSettings);

  if (settings !== syncedSettings) {
    const formIsClean = !isDirty;
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm(formFromSettings(settings));
      setValidationMessage("");
    }
  }

  const updateMutation = useMutation({
    mutationFn: updatePlaybackSettings,
    onSuccess: res => {
      if (res.error) {
        showActionFailed("save playback settings", res.message);
        return;
      }
      // The PUT echoes the same envelope the GET returns, so the response is
      // the authoritative catalog -- no merge, no refetch.
      queryClient.setQueryData<PlaybackSettingsQueryData>(
        [PLAYBACK_SETTINGS_KEY],
        res,
      );
      showSuccess("Playback settings saved");
    },
    onError: () => {
      showActionFailed(
        "save playback settings",
        "An unexpected error occurred",
      );
    },
  });

  const handleServerUploadMbpsChange = (value: string) => {
    const nextForm = { ...form, server_upload_mbps: parseMbpsInput(value) };
    setForm(nextForm);
    if (validationMessage) {
      setValidationMessage(
        isServerUploadOutOfRange(nextForm) ? SERVER_UPLOAD_VALIDATION_MESSAGE : "",
      );
    }
  };

  const handleHardwareChange = (value: string) => {
    if (!isHardwareAccelerationDevice(value)) return;
    startTransition(() => {
      setForm(current => ({
        ...current,
        hardware_acceleration_device: value,
      }));
    });
  };

  const resetForm = () => {
    setForm(formFromSettings(syncedSettings));
    setValidationMessage("");
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (isServerUploadOutOfRange(form)) {
      setValidationMessage(SERVER_UPLOAD_VALIDATION_MESSAGE);
      showValidationError(SERVER_UPLOAD_VALIDATION_MESSAGE);
      return;
    }
    setValidationMessage("");
    updateMutation.mutate(form);
  };

  const serverUploadMbpsInvalid =
    validationMessage === SERVER_UPLOAD_VALIDATION_MESSAGE &&
    isServerUploadOutOfRange(form);

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-6">
      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Server}
          title="Server"
          description="Applies to the whole server and every person using it."
        />
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Server upload bandwidth"
            description="The home server's outbound limit, used to cap stream quality recommendations."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={serverUploadMbpsId}>
                Server upload bandwidth (Mbps)
              </Label>
              <Input
                id={serverUploadMbpsId}
                name="server_upload_mbps"
                type="number"
                inputMode="decimal"
                min={0.1}
                step={0.1}
                value={form.server_upload_mbps ?? ""}
                onChange={event =>
                  handleServerUploadMbpsChange(event.target.value)
                }
                disabled={updateMutation.isPending}
                aria-invalid={serverUploadMbpsInvalid ? "true" : undefined}
                aria-describedby={
                  serverUploadMbpsInvalid
                    ? `${serverUploadMbpsId}-description ${statusId}`
                    : `${serverUploadMbpsId}-description`
                }
                className={SETTINGS_INPUT_CLASS}
              />
              <p
                id={`${serverUploadMbpsId}-description`}
                className="text-sm text-muted-foreground"
              >
                Leave blank if the server should be uncapped.
              </p>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Transcoding"
            description="Choose the hardware acceleration mode used for new transcodes."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={hardwareDeviceId}>Hardware acceleration</Label>
              <Select
                name="hardware_acceleration_device"
                value={form.hardware_acceleration_device}
                onValueChange={handleHardwareChange}
                disabled={updateMutation.isPending}
              >
                <SelectTrigger
                  id={hardwareDeviceId}
                  className={SETTINGS_SELECT_TRIGGER_CLASS}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                  {HARDWARE_OPTIONS.map(option => (
                    <SelectItem
                      key={option.value}
                      value={option.value}
                      className={SETTINGS_SELECT_ITEM_CLASS}
                    >
                      <span className="flex items-center gap-2">
                        {option.icon}
                        {option.label}
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                {hardwareOption(form.hardware_acceleration_device)?.description}
              </p>
              <EffectiveDeviceNotice settings={syncedSettings} />
            </div>
          </PlaybackSection>
        </CardContent>
        {/* Inside the card, so the one Save bar on this page is visibly the
            Server card's: the device and account cards above save as they
            change (design-system §3.7). */}
        <CardFooter className="block p-0">
          <SettingsSaveBar
            title="Server playback settings"
            isDirty={isDirty}
            statusId={statusId}
            statusMessage={
              validationMessage || "Applies to every device streaming from this server."
            }
            statusTone={validationMessage ? "error" : "neutral"}
            onReset={resetForm}
            isPending={updateMutation.isPending}
            embedded
          />
        </CardFooter>
      </Card>
    </form>
  );
}
