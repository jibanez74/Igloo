import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent, ReactNode } from "react";
import { Apple, CircuitBoard, Cpu, MonitorCog, Server } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
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
import { authUserQueryOpts, playbackSettingsQueryOpts } from "@/lib/query-opts";
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

export const Route = createFileRoute("/_auth/settings/playback")({
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
  const user =
    authData?.error === false && authData.data?.user ? authData.data.user : null;
  const { data, isLoading } = useQuery(playbackSettingsQueryOpts());

  const settings =
    data?.error === false && data.data?.settings ? data.data.settings : null;

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
      {user.is_admin && <ServerPlaybackForm settings={settings} />}
    </div>
  );
}

type ServerPlaybackFormProps = {
  settings: PlaybackSettingsType;
};

type PlaybackSettingsQueryData = {
  error: false;
  message?: string;
  data?: PlaybackSettingsResponseType;
};

/** The server-wide half of the page: admin-only, and the only part that saves. */
function ServerPlaybackForm({ settings }: ServerPlaybackFormProps) {
  const queryClient = useQueryClient();
  const serverUploadMbpsId = useId();
  const hardwareDeviceId = useId();
  const statusId = useId();

  const [form, setForm] = useState<UpdatePlaybackSettingsRequest>(() => ({
    server_upload_mbps: settings.server_upload_mbps,
    hardware_acceleration_device: settings.hardware_acceleration_device,
  }));
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [validationMessage, setValidationMessage] = useState("");
  const [, startTransition] = useTransition();

  if (settings !== syncedSettings) {
    const formIsClean =
      form.server_upload_mbps === syncedSettings.server_upload_mbps &&
      form.hardware_acceleration_device ===
        syncedSettings.hardware_acceleration_device;
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm({
        server_upload_mbps: settings.server_upload_mbps,
        hardware_acceleration_device: settings.hardware_acceleration_device,
      });
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
    setForm({
      server_upload_mbps: syncedSettings.server_upload_mbps,
      hardware_acceleration_device: syncedSettings.hardware_acceleration_device,
    });
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
                {
                  HARDWARE_OPTIONS.find(
                    option =>
                      option.value === form.hardware_acceleration_device,
                  )?.description
                }
              </p>
            </div>
          </PlaybackSection>
        </CardContent>
      </Card>

      <SettingsSaveBar
        title="Server playback settings"
        statusId={statusId}
        statusMessage={
          validationMessage || "Applies to every device streaming from this server."
        }
        statusTone={validationMessage ? "error" : "neutral"}
        onReset={resetForm}
        resetDisabled={updateMutation.isPending}
        isPending={updateMutation.isPending}
        className={cn("bg-card/70", MOTION_SETTINGS_SURFACE_CLASS)}
      />
    </form>
  );
}
