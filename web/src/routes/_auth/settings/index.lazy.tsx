import { createLazyFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Activity,
  Apple,
  CircuitBoard,
  Cpu,
  Download,
  Eye,
  EyeOff,
  FolderCog,
  Gauge,
  HardDrive,
  KeyRound,
  Logs,
  MonitorCog,
  RotateCcw,
  Save,
  Sliders,
  Wifi,
} from "lucide-react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { GENERAL_SETTINGS_KEY, PLAYBACK_SETTINGS_KEY } from "@/lib/constants";
import { updateGeneralSettings } from "@/lib/api";
import { generalSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  GeneralSettingsType,
  HardwareAccelerationDevice,
  UpdateGeneralSettingsRequest,
} from "@/types";

export const Route = createLazyFileRoute("/_auth/settings/")({
  component: GeneralSettings,
});

type HardwareOption = {
  value: HardwareAccelerationDevice;
  label: string;
  description: string;
  icon: ReactNode;
};

type SecretField =
  | "tmdb_key"
  | "jellyfin_token"
  | "spotify_client_id"
  | "spotify_client_secret";

const HARDWARE_OPTIONS: HardwareOption[] = [
  {
    value: "cpu",
    label: "CPU",
    description: "Use software encoding on the host CPU.",
    icon: <Cpu className="size-4 text-slate-300" aria-hidden="true" />,
  },
  {
    value: "apple",
    label: "Apple VideoToolbox",
    description: "Use Apple hardware acceleration on supported Macs.",
    icon: <Apple className="size-4 text-slate-300" aria-hidden="true" />,
  },
  {
    value: "nvidia",
    label: "NVIDIA NVENC",
    description: "Use NVIDIA GPU acceleration when available.",
    icon: <CircuitBoard className="size-4 text-slate-300" aria-hidden="true" />,
  },
  {
    value: "intel",
    label: "Intel Quick Sync",
    description: "Use Intel GPU acceleration when available.",
    icon: <MonitorCog className="size-4 text-slate-300" aria-hidden="true" />,
  },
];

function formFromSettings(
  settings: GeneralSettingsType,
): UpdateGeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    jellyfin_token: settings.jellyfin_token ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    hardware_acceleration_device: settings.hardware_acceleration_device,
    enable_logger: settings.enable_logger,
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    logs_dir: settings.logs_dir,
    transcode_dir: settings.transcode_dir,
    server_upload_mbps: settings.server_upload_mbps,
  };
}

function isHardwareAccelerationDevice(
  value: string,
): value is HardwareAccelerationDevice {
  return HARDWARE_OPTIONS.some(option => option.value === value);
}

function GeneralSettings() {
  const { data, isLoading } = useQuery(generalSettingsQueryOpts());

  const settings =
    data?.error === false && data.data?.settings ? data.data.settings : null;

  if (isLoading) {
    return <GeneralSettingsLoading />;
  }

  if (data?.error) {
    return (
      <div className="max-w-3xl animate-in duration-300 fade-in">
        <Card className="border-red-500/20 bg-red-500/10">
          <CardHeader>
            <CardTitle className="text-red-300">
              Settings unavailable
            </CardTitle>
            <CardDescription className="text-red-200/80">
              {data.message || "Failed to load general settings."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  if (!settings) {
    return null;
  }

  return <GeneralSettingsForm settings={settings} />;
}

type GeneralSettingsFormProps = {
  settings: GeneralSettingsType;
};

function GeneralSettingsForm({ settings }: GeneralSettingsFormProps) {
  const queryClient = useQueryClient();
  const tmdbKeyId = useId();
  const jellyfinTokenId = useId();
  const spotifyClientIdId = useId();
  const spotifyClientSecretId = useId();
  const hardwareDeviceId = useId();
  const staticDirId = useId();
  const logsDirId = useId();
  const transcodeDirId = useId();
  const serverUploadMbpsId = useId();
  const enableLoggerId = useId();
  const enableWatcherId = useId();
  const downloadImagesId = useId();

  const [form, setForm] = useState<UpdateGeneralSettingsRequest>(() =>
    formFromSettings(settings),
  );
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [validationMessage, setValidationMessage] = useState("");
  const [validationField, setValidationField] = useState<string | null>(null);
  const [, startTransition] = useTransition();

  if (settings !== syncedSettings) {
    setSyncedSettings(settings);
    setForm(formFromSettings(settings));
    setValidationMessage("");
    setValidationField(null);
  }

  const updateMutation = useMutation({
    mutationFn: updateGeneralSettings,
    onSuccess: res => {
      if (res.error) {
        showActionFailed("save settings", res.message);
        return;
      }

      queryClient.invalidateQueries({ queryKey: [GENERAL_SETTINGS_KEY] });
      queryClient.invalidateQueries({ queryKey: [PLAYBACK_SETTINGS_KEY] });

      if (res.data.restart_required) {
        showSuccess(
          "Settings saved",
          "Restart Igloo to apply service and path changes everywhere.",
        );
        return;
      }

      showSuccess("Settings saved");
    },
    onError: () => {
      showActionFailed("save settings", "An unexpected error occurred");
    },
  });

  const handleSecretChange = (field: SecretField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  const handleTextChange = (
    field: "static_dir" | "logs_dir" | "transcode_dir",
    value: string,
  ) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  const handleToggleChange = (
    field: "enable_logger" | "enable_watcher" | "download_images",
    value: boolean,
  ) => {
    startTransition(() => {
      setForm(current => ({ ...current, [field]: value }));
    });
  };

  const handleServerUploadMbpsChange = (value: string) => {
    const trimmed = value.trim();
    if (trimmed === "") {
      setForm(current => ({ ...current, server_upload_mbps: null }));
      return;
    }
    const parsed = Number.parseFloat(trimmed);
    setForm(current => ({
      ...current,
      server_upload_mbps: Number.isFinite(parsed) ? parsed : null,
    }));
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
    setForm(formFromSettings(settings));
    setValidationMessage("");
    setValidationField(null);
  };

  const validateForm = (): { field: string | null; message: string } => {
    if (form.static_dir.trim() === "") {
      return { field: "static_dir", message: "Static directory is required." };
    }

    if (form.logs_dir.trim() === "") {
      return { field: "logs_dir", message: "Logs directory is required." };
    }

    if (form.transcode_dir.trim() === "") {
      return {
        field: "transcode_dir",
        message: "Transcode directory is required.",
      };
    }

    if (
      form.server_upload_mbps != null &&
      (form.server_upload_mbps <= 0 || form.server_upload_mbps >= 100000)
    ) {
      return {
        field: "server_upload_mbps",
        message:
          "Server upload bandwidth must be greater than 0 and less than 100000 Mbps.",
      };
    }

    return { field: null, message: "" };
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const { field, message } = validateForm();
    if (message) {
      setValidationMessage(message);
      setValidationField(field);
      showValidationError(message);
      return;
    }

    setValidationMessage("");
    setValidationField(null);
    updateMutation.mutate({
      ...form,
      tmdb_key: form.tmdb_key.trim(),
      jellyfin_token: form.jellyfin_token.trim(),
      spotify_client_id: form.spotify_client_id.trim(),
      spotify_client_secret: form.spotify_client_secret.trim(),
      static_dir: form.static_dir.trim(),
      logs_dir: form.logs_dir.trim(),
      transcode_dir: form.transcode_dir.trim(),
    });
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="max-w-5xl animate-in space-y-6 duration-300 fade-in"
    >
      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Sliders className="size-5 text-amber-400" aria-hidden="true" />
            General Settings
          </CardTitle>
          <CardDescription className="text-slate-300">
            Configure application behavior, integrations, and local storage.
          </CardDescription>
        </CardHeader>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Gauge className="size-5 text-amber-400" aria-hidden="true" />
            Application Behavior
          </CardTitle>
          <CardDescription className="text-slate-300">
            Control background services and metadata handling.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 xl:grid-cols-3">
          <SwitchField
            id={enableLoggerId}
            label="File logging"
            description="Write application logs to the configured logs directory."
            checked={form.enable_logger}
            onCheckedChange={checked =>
              handleToggleChange("enable_logger", checked)
            }
            icon={<Logs className="size-5 text-amber-400" aria-hidden="true" />}
            disabled={updateMutation.isPending}
          />
          <SwitchField
            id={enableWatcherId}
            label="Library watcher"
            description="Watch configured libraries for file changes."
            checked={form.enable_watcher}
            onCheckedChange={checked =>
              handleToggleChange("enable_watcher", checked)
            }
            icon={
              <Activity
                className="size-5 text-emerald-400"
                aria-hidden="true"
              />
            }
            disabled={updateMutation.isPending}
          />
          <SwitchField
            id={downloadImagesId}
            label="Download images"
            description="Store downloaded artwork for local serving."
            checked={form.download_images}
            onCheckedChange={checked =>
              handleToggleChange("download_images", checked)
            }
            icon={
              <Download className="size-5 text-cyan-400" aria-hidden="true" />
            }
            disabled={updateMutation.isPending}
          />
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <MonitorCog className="size-5 text-amber-400" aria-hidden="true" />
            Playback Runtime
          </CardTitle>
          <CardDescription className="text-slate-300">
            Choose the hardware acceleration mode used for new transcodes.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2">
            <Label htmlFor={hardwareDeviceId}>Hardware acceleration</Label>
            <Select
              name="hardware_acceleration_device"
              value={form.hardware_acceleration_device}
              onValueChange={handleHardwareChange}
              disabled={updateMutation.isPending}
            >
              <SelectTrigger
                id={hardwareDeviceId}
                className="h-10 w-full border-slate-600 bg-slate-950/60 text-white shadow-none focus-visible:ring-amber-400/30"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-900 text-slate-100">
                {HARDWARE_OPTIONS.map(option => (
                  <SelectItem
                    key={option.value}
                    value={option.value}
                    className="focus:bg-slate-800 focus:text-white"
                  >
                    <span className="flex items-center gap-2">
                      {option.icon}
                      {option.label}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-sm text-slate-400">
              {
                HARDWARE_OPTIONS.find(
                  option =>
                    option.value === form.hardware_acceleration_device,
                )?.description
              }
            </p>
          </div>
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <KeyRound className="size-5 text-amber-400" aria-hidden="true" />
            External Services
          </CardTitle>
          <CardDescription className="text-slate-300">
            Manage credentials used for metadata and interoperability.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5 lg:grid-cols-2">
          <SecretInput
            id={tmdbKeyId}
            name="tmdb_key"
            label="TMDB API key"
            value={form.tmdb_key}
            onChange={value => handleSecretChange("tmdb_key", value)}
            disabled={updateMutation.isPending}
          />
          <SecretInput
            id={jellyfinTokenId}
            name="jellyfin_token"
            label="Jellyfin token"
            value={form.jellyfin_token}
            onChange={value => handleSecretChange("jellyfin_token", value)}
            disabled={updateMutation.isPending}
          />
          <SecretInput
            id={spotifyClientIdId}
            name="spotify_client_id"
            label="Spotify client ID"
            value={form.spotify_client_id}
            onChange={value => handleSecretChange("spotify_client_id", value)}
            disabled={updateMutation.isPending}
          />
          <SecretInput
            id={spotifyClientSecretId}
            name="spotify_client_secret"
            label="Spotify client secret"
            value={form.spotify_client_secret}
            onChange={value =>
              handleSecretChange("spotify_client_secret", value)
            }
            disabled={updateMutation.isPending}
          />
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <HardDrive className="size-5 text-amber-400" aria-hidden="true" />
            Local Storage
          </CardTitle>
          <CardDescription className="text-slate-300">
            Configure application-owned storage outside media libraries.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5 lg:grid-cols-3">
          <PathInput
            id={staticDirId}
            name="static_dir"
            label="Static directory"
            value={form.static_dir}
            onChange={value => handleTextChange("static_dir", value)}
            disabled={updateMutation.isPending}
            icon={<FolderCog className="size-4" aria-hidden="true" />}
            required
            invalid={validationField === "static_dir"}
          />
          <PathInput
            id={logsDirId}
            name="logs_dir"
            label="Logs directory"
            value={form.logs_dir}
            onChange={value => handleTextChange("logs_dir", value)}
            disabled={updateMutation.isPending}
            icon={<Logs className="size-4" aria-hidden="true" />}
            required
            invalid={validationField === "logs_dir"}
          />
          <PathInput
            id={transcodeDirId}
            name="transcode_dir"
            label="Transcode directory"
            value={form.transcode_dir}
            onChange={value => handleTextChange("transcode_dir", value)}
            disabled={updateMutation.isPending}
            icon={<HardDrive className="size-4" aria-hidden="true" />}
            required
            invalid={validationField === "transcode_dir"}
          />
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Wifi className="size-5 text-amber-400" aria-hidden="true" />
            Server Bandwidth
          </CardTitle>
          <CardDescription className="text-slate-300">
            Caps the server&apos;s outbound bandwidth when streaming to viewers
            outside your home network.
          </CardDescription>
        </CardHeader>
        <CardContent>
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
              aria-describedby={`${serverUploadMbpsId}-description`}
              aria-invalid={validationField === "server_upload_mbps"}
              className="h-10 border-slate-600 bg-slate-950/60 text-white placeholder:text-slate-500 focus-visible:ring-amber-400/30"
            />
            <p
              id={`${serverUploadMbpsId}-description`}
              className="text-sm text-slate-400"
            >
              Used to recommend a stream profile when viewers stream from
              outside the home network. Leave blank if uncapped.
            </p>
          </div>
        </CardContent>
      </Card>

      <div className="rounded-lg border border-slate-700/50 bg-slate-900/70 p-4 shadow-lg shadow-black/10 transition-colors duration-200 sm:flex sm:items-center sm:justify-between sm:gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-white">General settings</p>
          <p
            className={cn(
              "mt-1 text-sm transition-colors",
              validationMessage ? "text-red-300" : "text-slate-400",
            )}
            aria-live="polite"
          >
            {validationMessage ||
              "Saved settings are used by the backend on future requests."}
          </p>
        </div>
        <div className="mt-4 flex flex-col gap-2 sm:mt-0 sm:flex-row">
          <Button
            type="button"
            variant="outline"
            onClick={resetForm}
            disabled={!settings || updateMutation.isPending}
            className="border-slate-600 bg-slate-800/90 text-slate-100 hover:bg-slate-700 hover:text-white"
          >
            <RotateCcw className="size-4" aria-hidden="true" />
            Reset
          </Button>
          <Button
            type="submit"
            variant="accent"
            disabled={updateMutation.isPending}
          >
            {updateMutation.isPending ? (
              <Spinner className="size-4" aria-hidden="true" />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {updateMutation.isPending ? "Saving..." : "Save Settings"}
          </Button>
        </div>
      </div>
    </form>
  );
}

type SwitchFieldProps = {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  icon: ReactNode;
  disabled: boolean;
};

function SwitchField({
  id,
  label,
  description,
  checked,
  onCheckedChange,
  icon,
  disabled,
}: SwitchFieldProps) {
  const labelId = `${id}-label`;
  const descriptionId = `${id}-description`;

  return (
    <div className="rounded-lg border border-slate-700/50 bg-slate-900/50 p-4 transition-colors duration-200 hover:border-slate-600/70">
      <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
        <div
          className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-slate-800"
          aria-hidden="true"
        >
          {icon}
        </div>
        <p id={labelId} className="min-w-0 pt-1 text-sm font-medium text-white">
          {label}
        </p>
        <button
          type="button"
          role="switch"
          aria-checked={checked}
          aria-labelledby={labelId}
          aria-describedby={descriptionId}
          disabled={disabled}
          onClick={() => onCheckedChange(!checked)}
          className={cn(
            "relative mt-1 h-6 w-11 shrink-0 rounded-full border transition-colors duration-200 focus-visible:ring-2 focus-visible:ring-amber-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-900 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60",
            checked
              ? "border-amber-400 bg-amber-500"
              : "border-slate-600 bg-slate-700",
          )}
        >
          <span
            aria-hidden="true"
            className={cn(
              "absolute top-1/2 left-0 size-4 -translate-y-1/2 rounded-full bg-white shadow-sm transition-transform duration-200",
              checked ? "translate-x-5" : "translate-x-1",
            )}
          />
        </button>
        <p
          id={descriptionId}
          className="col-span-3 text-sm text-slate-400 min-[380px]:col-span-1 min-[380px]:col-start-2"
        >
          {description}
        </p>
      </div>
    </div>
  );
}

type SecretInputProps = {
  id: string;
  name: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
};

function SecretInput({
  id,
  name,
  label,
  value,
  onChange,
  disabled,
}: SecretInputProps) {
  const [visible, setVisible] = useState(false);
  const descriptionId = `${id}-description`;

  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Input
          id={id}
          name={name}
          type={visible ? "text" : "password"}
          value={value}
          onChange={event => onChange(event.target.value)}
          disabled={disabled}
          aria-describedby={descriptionId}
          autoComplete="off"
          className="h-10 border-slate-600 bg-slate-950/60 pr-11 text-white placeholder:text-slate-500 focus-visible:ring-amber-400/30"
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => setVisible(current => !current)}
          disabled={disabled}
          aria-label={visible ? `Hide ${label}` : `Show ${label}`}
          className="absolute top-1 right-1 text-slate-400 hover:bg-slate-800 hover:text-white"
        >
          {visible ? (
            <EyeOff className="size-4" aria-hidden="true" />
          ) : (
            <Eye className="size-4" aria-hidden="true" />
          )}
        </Button>
      </div>
      <p id={descriptionId} className="text-sm text-slate-400">
        Leave blank to clear this value.
      </p>
    </div>
  );
}

type PathInputProps = {
  id: string;
  name: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  icon: ReactNode;
  required?: boolean;
  invalid?: boolean;
};

function PathInput({
  id,
  name,
  label,
  value,
  onChange,
  disabled,
  icon,
  required = false,
  invalid = false,
}: PathInputProps) {
  const descriptionId = `${id}-description`;

  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <span className="absolute top-1/2 left-3 z-10 -translate-y-1/2 text-slate-400">
          {icon}
        </span>
        <Input
          id={id}
          name={name}
          type="text"
          value={value}
          onChange={event => onChange(event.target.value)}
          disabled={disabled}
          required={required}
          aria-required={required || undefined}
          aria-invalid={invalid || undefined}
          aria-describedby={descriptionId}
          className="h-10 border-slate-600 bg-slate-950/60 pl-10 text-white placeholder:text-slate-500 focus-visible:ring-amber-400/30"
        />
      </div>
      <p id={descriptionId} className="text-sm text-slate-400">
        This path must be readable by the server.
      </p>
    </div>
  );
}

function GeneralSettingsLoading() {
  const loadingId = useId();

  return (
    <div
      className="max-w-5xl animate-in space-y-6 duration-300 fade-in"
      role="status"
      aria-labelledby={loadingId}
    >
      <Card className="border-slate-700/50 bg-slate-800/30">
        <CardContent className="flex min-h-40 items-center justify-center">
          <div className="flex items-center gap-3 text-slate-300">
            <Spinner className="size-5 text-amber-400" aria-hidden="true" />
            <span id={loadingId}>Loading general settings...</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
