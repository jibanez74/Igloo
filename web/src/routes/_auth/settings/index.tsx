import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Activity,
  Disc3,
  Download,
  Eye,
  EyeOff,
  FolderCog,
  Gauge,
  HardDrive,
  KeyRound,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SwitchField from "@/components/settings/SwitchField";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import { useIsMiniPlayerVisible } from "@/hooks/useIsMiniPlayerVisible";
import {
  MINI_PLAYER_CLEARANCE_BOTTOM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
} from "@/lib/constants";
import { updateGeneralSettings } from "@/lib/api";
import { trimmedOrNull } from "@/lib/nullable";
import { generalSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  GeneralSettingsResponseType,
  GeneralSettingsType,
  UpdateGeneralSettingsRequest,
} from "@/types";
import { routeHead } from "@/lib/route-head";
import { apiErrorMessage } from "@/lib/is-api-failure";

const GENERAL_SETTINGS_HEAD = routeHead("General Settings");

export const Route = createFileRoute("/_auth/settings/")({
  head: () => GENERAL_SETTINGS_HEAD,
  component: GeneralSettings,
});

type SecretField =
  | "tmdb_key"
  | "spotify_client_id"
  | "spotify_client_secret";

function formFromSettings(
  settings: GeneralSettingsType,
): UpdateGeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    enable_apple_lookups: settings.enable_apple_lookups,
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    transcode_dir: settings.transcode_dir,
  };
}

function formsMatchSettings(
  form: UpdateGeneralSettingsRequest,
  settings: GeneralSettingsType,
) {
  const settingsForm = formFromSettings(settings);
  return (
    form.tmdb_key === settingsForm.tmdb_key &&
    form.spotify_client_id === settingsForm.spotify_client_id &&
    form.spotify_client_secret === settingsForm.spotify_client_secret &&
    form.enable_apple_lookups === settingsForm.enable_apple_lookups &&
    form.enable_watcher === settingsForm.enable_watcher &&
    form.download_images === settingsForm.download_images &&
    form.static_dir === settingsForm.static_dir &&
    form.transcode_dir === settingsForm.transcode_dir
  );
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
        message={apiErrorMessage(data, "Failed to load general settings.")}
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
  const spotifyClientIdId = useId();
  const spotifyClientSecretId = useId();
  const appleLookupsId = useId();
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

  const isDirty = !formsMatchSettings(form, syncedSettings);

  if (settings !== syncedSettings) {
    const formIsClean = !isDirty;
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm(formFromSettings(settings));
      setValidationMessage("");
      setValidationField(null);
    }
  }

  const settingsKey = generalSettingsQueryOpts().queryKey;
  // Undoes the optimistic write when the save is refused or fails.
  const restoreSettings = (
    previousData: ApiResponseType<GeneralSettingsResponseType> | undefined,
  ) => {
    if (previousData) queryClient.setQueryData(settingsKey, previousData);
  };
  const updateMutation = useMutation({
    mutationFn: updateGeneralSettings,
    onMutate: async nextSettings => {
      await queryClient.cancelQueries({ queryKey: settingsKey });

      const previousData = queryClient.getQueryData(settingsKey);

      if (previousData?.error === false) {
        queryClient.setQueryData(settingsKey, {
          ...previousData,
          data: {
            ...previousData.data,
            settings: {
              ...previousData.data.settings,
              tmdb_key: trimmedOrNull(nextSettings.tmdb_key),
              spotify_client_id: trimmedOrNull(nextSettings.spotify_client_id),
              spotify_client_secret: trimmedOrNull(
                nextSettings.spotify_client_secret,
              ),
              enable_apple_lookups: nextSettings.enable_apple_lookups,
              enable_watcher: nextSettings.enable_watcher,
              download_images: nextSettings.download_images,
              static_dir: nextSettings.static_dir,
              transcode_dir: nextSettings.transcode_dir,
            },
          },
        });
      }

      return { previousData };
    },
    onSuccess: (res, _nextSettings, context) => {
      if (res.error) {
        restoreSettings(context?.previousData);
        showActionFailed("save settings", res);
        return;
      }

      // The PUT answers with the saved settings, so the cache takes them
      // as they are rather than refetching.
      queryClient.setQueryData(settingsKey, {
        error: false,
        message: res.message,
        data: {
          settings: res.data.settings,
        },
      });

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
      restoreSettings(context?.previousData);
      showActionFailed("save settings", "An unexpected error occurred");
    },
  });

  const handleSecretChange = (field: SecretField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
  };

  // Editing the flagged field drops its error, including when the edit
  // restores the saved value and leaves Reset with nothing to do.
  const clearFieldError = (field: string) => {
    if (validationField !== field) return;
    setValidationField(null);
    setValidationMessage("");
  };

  const handleTextChange = (
    field: "static_dir" | "transcode_dir",
    value: string,
  ) => {
    setForm(current => ({ ...current, [field]: value }));
    clearFieldError(field);
  };

  const handleToggleChange = (
    field: "enable_watcher" | "download_images" | "enable_apple_lookups",
    value: boolean,
  ) => {
    startTransition(() => {
      setForm(current => ({ ...current, [field]: value }));
    });
  };

  const resetForm = () => {
    setForm(formFromSettings(syncedSettings));
    setValidationMessage("");
    setValidationField(null);
  };

  const validateForm = (): { field: string | null; message: string } => {
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
      if (field) {
        (event.currentTarget.elements.namedItem(field) as HTMLElement | null)?.focus();
      }
      return;
    }

    setValidationMessage("");
    setValidationField(null);
    updateMutation.mutate({
      ...form,
      tmdb_key: form.tmdb_key.trim(),
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
            description="Apply added, changed, and removed library files as they happen."
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

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={KeyRound}
          title="External Services"
          description="Manage the services and credentials used for metadata."
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
          <ServiceSection title="Apple Music">
            <SwitchField
              id={appleLookupsId}
              label="Apple Music lookups"
              description="Match albums from iTunes purchases against the Apple Music catalog during music scans. No account needed; applies from the next scan."
              checked={form.enable_apple_lookups}
              onCheckedChange={checked =>
                handleToggleChange("enable_apple_lookups", checked)
              }
              icon={<Disc3 className="size-5 text-primary" aria-hidden="true" />}
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

      <SettingsSaveBar
        title="General settings"
        isDirty={isDirty}
        statusMessage={
          validationMessage ||
          "Saved settings are used by the backend on future requests."
        }
        statusTone={validationMessage ? "error" : "neutral"}
        onReset={resetForm}
        isPending={updateMutation.isPending}
        stickyClassName={
          isMiniPlayerVisible ? MINI_PLAYER_CLEARANCE_BOTTOM_CLASS : "bottom-4"
        }
        className={MOTION_SETTINGS_SURFACE_CLASS}
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
        The server must be able to write to this directory.
      </p>
    </div>
  );
}

