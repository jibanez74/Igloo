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
  MonitorCog,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
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
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import { useIsMiniPlayerVisible } from "@/hooks/useIsMiniPlayerVisible";
import {
  FOCUS_VISIBLE_RING_CLASS,
  GENERAL_SETTINGS_KEY,
  MINI_PLAYER_CLEARANCE_BOTTOM_CLASS,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
  SETTINGS_SELECT_CONTENT_CLASS,
  SETTINGS_SELECT_ITEM_CLASS,
  SETTINGS_SELECT_TRIGGER_CLASS,
} from "@/lib/constants";
import { updateGeneralSettings } from "@/lib/api";
import { generalSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  GeneralSettingsResponseType,
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
  | "immich_api_key"
  | "jellyfin_api_key"
  | "spotify_client_id"
  | "spotify_client_secret";

type BaseURLField = "immich_base_url" | "jellyfin_base_url";

type GeneralSettingsQueryData = ApiResponseType<GeneralSettingsResponseType>;

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

function formFromSettings(
  settings: GeneralSettingsType,
): UpdateGeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    immich_base_url: settings.immich_base_url ?? "",
    immich_api_key: settings.immich_api_key ?? "",
    jellyfin_base_url: settings.jellyfin_base_url ?? "",
    jellyfin_api_key: settings.jellyfin_api_key ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    hardware_acceleration_device: settings.hardware_acceleration_device,
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    transcode_dir: settings.transcode_dir,
    server_upload_mbps: settings.server_upload_mbps,
  };
}

function formsMatchSettings(
  form: UpdateGeneralSettingsRequest,
  settings: GeneralSettingsType,
) {
  const settingsForm = formFromSettings(settings);
  return (
    form.tmdb_key === settingsForm.tmdb_key &&
    form.immich_base_url === settingsForm.immich_base_url &&
    form.immich_api_key === settingsForm.immich_api_key &&
    form.jellyfin_base_url === settingsForm.jellyfin_base_url &&
    form.jellyfin_api_key === settingsForm.jellyfin_api_key &&
    form.spotify_client_id === settingsForm.spotify_client_id &&
    form.spotify_client_secret === settingsForm.spotify_client_secret &&
    form.hardware_acceleration_device ===
      settingsForm.hardware_acceleration_device &&
    form.enable_watcher === settingsForm.enable_watcher &&
    form.download_images === settingsForm.download_images &&
    form.static_dir === settingsForm.static_dir &&
    form.transcode_dir === settingsForm.transcode_dir &&
    form.server_upload_mbps === settingsForm.server_upload_mbps
  );
}

function isHardwareAccelerationDevice(
  value: string,
): value is HardwareAccelerationDevice {
  return HARDWARE_OPTIONS.some(option => option.value === value);
}

function isOptionalHTTPBaseURL(value: string) {
  const trimmed = value.trim();
  if (trimmed === "") {
    return true;
  }

  try {
    const url = new URL(trimmed);
    return (
      (url.protocol === "http:" || url.protocol === "https:") &&
      url.host !== ""
    );
  } catch {
    return false;
  }
}

function optionalSetting(value: string) {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function GeneralSettings() {
  const { data, isLoading } = useQuery(generalSettingsQueryOpts());

  const settings =
    data?.error === false && data.data?.settings ? data.data.settings : null;

  if (isLoading) {
    return <SettingsLoadingCard label="Loading general settings..." />;
  }

  if (data?.error) {
    return (
      <SettingsErrorCard
        title="Settings unavailable"
        message={data.message || "Failed to load general settings."}
      />
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
  const isMiniPlayerVisible = useIsMiniPlayerVisible();
  const tmdbKeyId = useId();
  const jellyfinBaseUrlId = useId();
  const jellyfinApiKeyId = useId();
  const immichBaseUrlId = useId();
  const immichApiKeyId = useId();
  const spotifyClientIdId = useId();
  const spotifyClientSecretId = useId();
  const hardwareDeviceId = useId();
  const staticDirId = useId();
  const transcodeDirId = useId();
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
    const formIsClean = formsMatchSettings(form, syncedSettings);
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm(formFromSettings(settings));
      setValidationMessage("");
      setValidationField(null);
    }
  }

  const updateMutation = useMutation({
    mutationFn: updateGeneralSettings,
    onMutate: async nextSettings => {
      await queryClient.cancelQueries({ queryKey: [GENERAL_SETTINGS_KEY] });

      const previousData = queryClient.getQueryData<GeneralSettingsQueryData>([
        GENERAL_SETTINGS_KEY,
      ]);

      queryClient.setQueryData<GeneralSettingsQueryData>(
        [GENERAL_SETTINGS_KEY],
        current => {
          if (current?.error !== false || !current.data?.settings) {
            return current;
          }

          return {
            ...current,
            data: {
              ...current.data,
              settings: {
                ...current.data.settings,
                tmdb_key: optionalSetting(nextSettings.tmdb_key),
                immich_base_url: optionalSetting(nextSettings.immich_base_url),
                immich_api_key: optionalSetting(nextSettings.immich_api_key),
                jellyfin_base_url: optionalSetting(
                  nextSettings.jellyfin_base_url,
                ),
                jellyfin_api_key: optionalSetting(nextSettings.jellyfin_api_key),
                spotify_client_id: optionalSetting(
                  nextSettings.spotify_client_id,
                ),
                spotify_client_secret: optionalSetting(
                  nextSettings.spotify_client_secret,
                ),
                hardware_acceleration_device:
                  nextSettings.hardware_acceleration_device,
                enable_watcher: nextSettings.enable_watcher,
                download_images: nextSettings.download_images,
                static_dir: nextSettings.static_dir,
                transcode_dir: nextSettings.transcode_dir,
                server_upload_mbps: nextSettings.server_upload_mbps,
              },
            },
          };
        },
      );

      return { previousData };
    },
    onSuccess: (res, _nextSettings, context) => {
      if (res.error) {
        if (context?.previousData) {
          queryClient.setQueryData([GENERAL_SETTINGS_KEY], context.previousData);
        }
        showActionFailed("save settings", res.message);
        return;
      }

      queryClient.setQueryData<GeneralSettingsQueryData>(
        [GENERAL_SETTINGS_KEY],
        {
          error: false,
          message: res.message,
          data: {
            settings: res.data.settings,
          },
        },
      );
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
    onError: (_error, _nextSettings, context) => {
      if (context?.previousData) {
        queryClient.setQueryData([GENERAL_SETTINGS_KEY], context.previousData);
      }
      showActionFailed("save settings", "An unexpected error occurred");
    },
  });

  const handleSecretChange = (field: SecretField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  const handleBaseURLChange = (field: BaseURLField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  const handleTextChange = (
    field: "static_dir" | "transcode_dir",
    value: string,
  ) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  const handleToggleChange = (
    field: "enable_watcher" | "download_images",
    value: boolean,
  ) => {
    startTransition(() => {
      setForm(current => ({ ...current, [field]: value }));
    });
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
    setValidationField(null);
  };

  const validateForm = (): { field: string | null; message: string } => {
    if (!isOptionalHTTPBaseURL(form.jellyfin_base_url)) {
      return {
        field: "jellyfin_base_url",
        message: "Jellyfin base URL must start with http:// or https://.",
      };
    }

    if (!isOptionalHTTPBaseURL(form.immich_base_url)) {
      return {
        field: "immich_base_url",
        message: "Immich base URL must start with http:// or https://.",
      };
    }

    if (form.static_dir.trim() === "") {
      return { field: "static_dir", message: "Static directory is required." };
    }

    if (form.transcode_dir.trim() === "") {
      return {
        field: "transcode_dir",
        message: "Transcode directory is required.",
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
      immich_base_url: form.immich_base_url.trim(),
      immich_api_key: form.immich_api_key.trim(),
      jellyfin_base_url: form.jellyfin_base_url.trim(),
      jellyfin_api_key: form.jellyfin_api_key.trim(),
      spotify_client_id: form.spotify_client_id.trim(),
      spotify_client_secret: form.spotify_client_secret.trim(),
      static_dir: form.static_dir.trim(),
      transcode_dir: form.transcode_dir.trim(),
    });
  };

  return (
    // Deliberately one form with a single "Save Settings" action. Its masked
    // API-key fields (type="password" in SecretInput) trip Chrome's
    // password-manager "multiple forms" heuristic (a benign verbose advisory,
    // not an error); splitting the form would break the single-save UX.
    <form
      onSubmit={handleSubmit}
      noValidate
      // scroll-mb on every focusable control: the sticky save bar overlays the
      // bottom of the viewport, so keyboard-focus auto-scroll must clear it
      // (bar height + its bottom offset, largest when stacked on mobile with
      // the mini player visible).
      className="@container max-w-5xl space-y-6 [&_:is(button,input,select)]:scroll-mb-72 sm:[&_:is(button,input,select)]:scroll-mb-44"
    >
      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Gauge}
          title="Application Behavior"
          description="Control background services and metadata handling."
        />
        <CardContent className="grid gap-4 @2xl:grid-cols-2">
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
                className="size-5 text-success"
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
              <Download className="size-5 text-primary" aria-hidden="true" />
            }
            disabled={updateMutation.isPending}
          />
        </CardContent>
      </Card>

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={MonitorCog}
          title="Playback Runtime"
          description="Choose the hardware acceleration mode used for new transcodes."
        />
        <CardContent>
          <div className="grid gap-2">
            <Label htmlFor={hardwareDeviceId}>Hardware acceleration</Label>
            {/*
              Radix renders a visually-hidden aria-hidden <select name> here for
              native form integration; it's never submitted (this form is
              JS-controlled). Chrome's Issues panel flags it "no label" — a
              false-positive on an aria-hidden field, not fixable without ejecting
              Radix. Keep the name so the field stays identified.
            */}
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
        </CardContent>
      </Card>

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={KeyRound}
          title="External Services"
          description="Manage credentials used for metadata and interoperability."
        />
        <CardContent className="divide-y divide-border/50">
          <ServiceSection title="TMDB">
            <SecretInput
              id={tmdbKeyId}
              name="tmdb_key"
              label="TMDB API key"
              value={form.tmdb_key}
              onChange={value => handleSecretChange("tmdb_key", value)}
              disabled={updateMutation.isPending}
            />
          </ServiceSection>
          <ServiceSection title="Jellyfin">
            <URLInput
              id={jellyfinBaseUrlId}
              name="jellyfin_base_url"
              label="Jellyfin base URL"
              value={form.jellyfin_base_url}
              onChange={value =>
                handleBaseURLChange("jellyfin_base_url", value)
              }
              disabled={updateMutation.isPending}
              invalid={validationField === "jellyfin_base_url"}
            />
            <SecretInput
              id={jellyfinApiKeyId}
              name="jellyfin_api_key"
              label="Jellyfin API key"
              value={form.jellyfin_api_key}
              onChange={value => handleSecretChange("jellyfin_api_key", value)}
              disabled={updateMutation.isPending}
            />
          </ServiceSection>
          <ServiceSection title="Immich">
            <URLInput
              id={immichBaseUrlId}
              name="immich_base_url"
              label="Immich base URL"
              value={form.immich_base_url}
              onChange={value => handleBaseURLChange("immich_base_url", value)}
              disabled={updateMutation.isPending}
              invalid={validationField === "immich_base_url"}
            />
            <SecretInput
              id={immichApiKeyId}
              name="immich_api_key"
              label="Immich API key"
              value={form.immich_api_key}
              onChange={value => handleSecretChange("immich_api_key", value)}
              disabled={updateMutation.isPending}
            />
          </ServiceSection>
          <ServiceSection title="Spotify">
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
          </ServiceSection>
        </CardContent>
      </Card>

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={HardDrive}
          title="Local Storage"
          description="Configure application-owned storage outside media libraries."
        />
        <CardContent className="grid gap-5 @2xl:grid-cols-2">
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

      <SettingsSaveBar
        title="General settings"
        statusMessage={
          validationMessage ||
          "Saved settings are used by the backend on future requests."
        }
        statusTone={validationMessage ? "error" : "neutral"}
        onReset={resetForm}
        resetDisabled={updateMutation.isPending}
        isPending={updateMutation.isPending}
        className={cn(
          "sticky z-10 bg-card/95 backdrop-blur-md supports-backdrop-filter:bg-card/85",
          isMiniPlayerVisible ? MINI_PLAYER_CLEARANCE_BOTTOM_CLASS : "bottom-4",
          MOTION_SETTINGS_SURFACE_CLASS,
        )}
      />
    </form>
  );
}

type ServiceSectionProps = {
  title: string;
  children: ReactNode;
};

function ServiceSection({ title, children }: ServiceSectionProps) {
  return (
    <section className="py-5 first:pt-0 last:pb-0">
      <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      <div className="mt-3 grid gap-5 @2xl:grid-cols-2">{children}</div>
    </section>
  );
}

type SwitchFieldProps = {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  icon: ReactNode;
  disabled?: boolean;
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
    <div
      className={cn(
        "rounded-lg border border-border/50 bg-card/50 p-4 hover:border-border/70",
        MOTION_SETTINGS_SURFACE_CLASS,
      )}
    >
      <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
        <div
          className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted"
          aria-hidden="true"
        >
          {icon}
        </div>
        <p id={labelId} className="min-w-0 pt-1 text-sm font-medium text-foreground">
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
            "relative mt-1 h-6 w-11 shrink-0 rounded-full border disabled:cursor-not-allowed disabled:opacity-60",
            FOCUS_VISIBLE_RING_CLASS,
            MOTION_SETTINGS_SURFACE_CLASS,
            checked
              ? "border-primary bg-primary"
              : "border-border bg-accent",
          )}
        >
          <span
            aria-hidden="true"
            className={cn(
              "absolute top-1/2 left-0 size-4 -translate-y-1/2 rounded-full bg-white shadow-sm",
              MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
              checked ? "translate-x-5" : "translate-x-1",
            )}
          />
        </button>
        <p
          id={descriptionId}
          className="col-span-3 text-sm text-muted-foreground min-[380px]:col-span-1 min-[380px]:col-start-2"
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

type URLInputProps = {
  id: string;
  name: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  invalid: boolean;
};

function URLInput({
  id,
  name,
  label,
  value,
  onChange,
  disabled,
  invalid,
}: URLInputProps) {
  const descriptionId = `${id}-description`;

  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        name={name}
        type="url"
        inputMode="url"
        value={value}
        onChange={event => onChange(event.target.value)}
        disabled={disabled}
        aria-describedby={descriptionId}
        aria-invalid={invalid || undefined}
        autoComplete="off"
        className={SETTINGS_INPUT_CLASS}
      />
      <p id={descriptionId} className="text-sm text-muted-foreground">
        Use http:// or https://. Leave blank to clear this value.
      </p>
    </div>
  );
}

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
          className={cn(SETTINGS_INPUT_CLASS, "pr-11")}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => setVisible(current => !current)}
          disabled={disabled}
          aria-label={visible ? `Hide ${label}` : `Show ${label}`}
          className="absolute top-1 right-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          {visible ? (
            <EyeOff className="size-4" aria-hidden="true" />
          ) : (
            <Eye className="size-4" aria-hidden="true" />
          )}
        </Button>
      </div>
      <p id={descriptionId} className="text-sm text-muted-foreground">
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
        <span className="absolute top-1/2 left-3 z-10 -translate-y-1/2 text-muted-foreground">
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
          className={cn(SETTINGS_INPUT_CLASS, "pl-10")}
        />
      </div>
      <p id={descriptionId} className="text-sm text-muted-foreground">
        This path must be readable by the server.
      </p>
    </div>
  );
}

