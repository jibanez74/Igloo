import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Apple,
  CircuitBoard,
  Cpu,
  Gauge,
  MonitorCog,
  Sliders,
} from "lucide-react";
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
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import {
  LANGUAGE_NAMES,
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
  SETTINGS_SELECT_CONTENT_CLASS,
  SETTINGS_SELECT_ITEM_CLASS,
  SETTINGS_SELECT_TRIGGER_CLASS,
  SUBTITLE_OFF_VALUE,
} from "@/lib/constants";
import { updatePlaybackSettings } from "@/lib/api";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import { authUserQueryOpts, playbackSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  HardwareAccelerationDevice,
  PlaybackSettingsResponseType,
  PlaybackSettingsType,
  UpdatePlaybackSettingsRequest,
} from "@/types";

export const Route = createFileRoute("/_auth/settings/playback")({
  component: PlaybackSettings,
});

const NO_SELECTION_VALUE = "__none__";
const DOWNLOAD_SPEED_MAX_MBPS = 10_000;
const SERVER_UPLOAD_MAX_MBPS = 100_000;
const LANGUAGE_CODE_PATTERN = /^[a-z]{2,3}$/;
const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  `Download speed must be between 0 and ${DOWNLOAD_SPEED_MAX_MBPS} Mbps.`;
const SERVER_UPLOAD_VALIDATION_MESSAGE =
  `Server upload bandwidth must be greater than 0 and less than ${SERVER_UPLOAD_MAX_MBPS} Mbps.`;

const SORTED_LANGUAGE_ENTRIES = Object.entries(LANGUAGE_NAMES).sort(
  ([, a], [, b]) => a.localeCompare(b),
);

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

function formFromSettings(
  settings: PlaybackSettingsType,
): UpdatePlaybackSettingsRequest {
  const form: UpdatePlaybackSettingsRequest = {
    preferred_profile: settings.preferred_profile,
    download_mbps: settings.download_mbps,
    preferred_audio_language: settings.preferred_audio_language,
    preferred_subtitle_language: settings.preferred_subtitle_language,
  };

  if (settings.is_admin) {
    form.server_upload_mbps = settings.server_upload_mbps;
    form.hardware_acceleration_device = settings.hardware_acceleration_device;
  }

  return form;
}

function formsMatchSettings(
  form: UpdatePlaybackSettingsRequest,
  settings: PlaybackSettingsType,
) {
  const settingsForm = formFromSettings(settings);
  return (
    form.preferred_profile === settingsForm.preferred_profile &&
    form.download_mbps === settingsForm.download_mbps &&
    form.preferred_audio_language === settingsForm.preferred_audio_language &&
    form.preferred_subtitle_language ===
      settingsForm.preferred_subtitle_language &&
    form.server_upload_mbps === settingsForm.server_upload_mbps &&
    form.hardware_acceleration_device ===
      settingsForm.hardware_acceleration_device
  );
}

function isDownloadSpeedOutOfRange(form: UpdatePlaybackSettingsRequest) {
  return (
    form.download_mbps != null &&
    (form.download_mbps <= 0 || form.download_mbps >= DOWNLOAD_SPEED_MAX_MBPS)
  );
}

function isServerUploadOutOfRange(form: UpdatePlaybackSettingsRequest) {
  return (
    form.server_upload_mbps != null &&
    (form.server_upload_mbps <= 0 ||
      form.server_upload_mbps >= SERVER_UPLOAD_MAX_MBPS)
  );
}

function validatePlaybackSettingsForm(
  form: UpdatePlaybackSettingsRequest,
  settings: PlaybackSettingsType,
) {
  if (isDownloadSpeedOutOfRange(form)) {
    return DOWNLOAD_SPEED_VALIDATION_MESSAGE;
  }
  if (
    form.preferred_profile != null &&
    !settings.profiles.some(p => p.id === form.preferred_profile)
  ) {
    return "Selected profile is not available.";
  }
  if (
    form.preferred_audio_language != null &&
    !LANGUAGE_CODE_PATTERN.test(form.preferred_audio_language)
  ) {
    return "Audio language must be a 2- or 3-letter lowercase code.";
  }
  if (
    form.preferred_subtitle_language != null &&
    form.preferred_subtitle_language !== SUBTITLE_OFF_VALUE &&
    !LANGUAGE_CODE_PATTERN.test(form.preferred_subtitle_language)
  ) {
    return "Subtitle language must be a 2- or 3-letter lowercase code.";
  }
  if (settings.is_admin && isServerUploadOutOfRange(form)) {
    return SERVER_UPLOAD_VALIDATION_MESSAGE;
  }
  return "";
}

function PlaybackSettings() {
  const { data: authData, isLoading: authLoading } = useQuery(
    authUserQueryOpts(),
  );
  const userId =
    authData?.error === false && authData.data?.user
      ? authData.data.user.id
      : null;
  const { data, isLoading } = useQuery({
    ...playbackSettingsQueryOpts(userId ?? 0),
    enabled: userId !== null,
  });

  const settings =
    data?.error === false && data.data?.settings ? data.data.settings : null;

  if (authLoading || isLoading) {
    return <SettingsLoadingCard label="Loading playback settings..." />;
  }

  if (authData?.error || userId === null) {
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

  return <PlaybackSettingsForm settings={settings} userId={userId} />;
}

type PlaybackSettingsFormProps = {
  settings: PlaybackSettingsType;
  userId: number;
};

type PlaybackSettingsQueryData = ApiResponseType<PlaybackSettingsResponseType>;

function PlaybackSettingsForm({ settings, userId }: PlaybackSettingsFormProps) {
  const queryClient = useQueryClient();
  const downloadMbpsId = useId();
  const preferredProfileId = useId();
  const recommendationId = useId();
  const recommendationTitleId = useId();
  const serverUploadMbpsId = useId();
  const hardwareDeviceId = useId();
  const statusId = useId();
  const preferredAudioLanguageId = useId();
  const preferredSubtitleLanguageId = useId();

  const [form, setForm] = useState<UpdatePlaybackSettingsRequest>(() =>
    formFromSettings(settings),
  );
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [validationMessage, setValidationMessage] = useState("");
  const [, startTransition] = useTransition();

  if (settings !== syncedSettings) {
    const formIsClean = formsMatchSettings(form, syncedSettings);
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm(formFromSettings(settings));
      setValidationMessage("");
    }
  }

  const updateMutation = useMutation({
    mutationFn: updatePlaybackSettings,
    onSuccess: (res, nextSettings) => {
      if (res.error) {
        showActionFailed("save playback settings", res.message);
        return;
      }
      queryClient.setQueryData<PlaybackSettingsQueryData>(
        [PLAYBACK_SETTINGS_KEY, userId],
        current => {
          const currentSettings =
            current?.error === false && current.data?.settings
              ? current.data.settings
              : syncedSettings;
          const serverUpload =
            nextSettings.server_upload_mbps !== undefined
              ? nextSettings.server_upload_mbps
              : currentSettings.server_upload_mbps;
          const hardwareDevice =
            nextSettings.hardware_acceleration_device !== undefined
              ? nextSettings.hardware_acceleration_device
              : currentSettings.hardware_acceleration_device;

          return {
            error: false,
            message: res.message,
            data: {
              settings: {
                ...currentSettings,
                ...res.data.settings,
                server_upload_mbps: serverUpload,
                hardware_acceleration_device: hardwareDevice,
              },
            },
          };
        },
      );
      queryClient.invalidateQueries({ queryKey: [PLAYBACK_SETTINGS_KEY] });
      showSuccess("Playback settings saved");
    },
    onError: () => {
      showActionFailed(
        "save playback settings",
        "An unexpected error occurred",
      );
    },
  });

  const handleDownloadMbpsChange = (value: string) => {
    const trimmed = value.trim();
    let downloadMbps: number | null = null;
    if (trimmed === "") {
      downloadMbps = null;
    } else {
      const parsed = Number.parseFloat(trimmed);
      downloadMbps = Number.isFinite(parsed) ? parsed : null;
    }

    const nextForm = { ...form, download_mbps: downloadMbps };
    setForm(nextForm);
    if (validationMessage) {
      setValidationMessage(validatePlaybackSettingsForm(nextForm, settings));
    }
  };

  const handleServerUploadMbpsChange = (value: string) => {
    const trimmed = value.trim();
    let serverUploadMbps: number | null = null;
    if (trimmed === "") {
      serverUploadMbps = null;
    } else {
      const parsed = Number.parseFloat(trimmed);
      serverUploadMbps = Number.isFinite(parsed) ? parsed : null;
    }

    const nextForm = { ...form, server_upload_mbps: serverUploadMbps };
    setForm(nextForm);
    if (validationMessage) {
      setValidationMessage(validatePlaybackSettingsForm(nextForm, settings));
    }
  };

  const handleProfileChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_profile: value === NO_SELECTION_VALUE ? null : value,
      }));
    });
  };

  const handleAudioLanguageChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_audio_language:
          value === NO_SELECTION_VALUE ? null : value,
      }));
    });
  };

  const handleSubtitleLanguageChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_subtitle_language:
          value === NO_SELECTION_VALUE ? null : value,
      }));
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
  };

  const validateForm = () => {
    return validatePlaybackSettingsForm(form, settings);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const message = validateForm();
    if (message) {
      setValidationMessage(message);
      showValidationError(message);
      return;
    }
    setValidationMessage("");
    updateMutation.mutate(form);
  };

  const recommendedId = recommendedProfileId(
    settings.profiles,
    form.download_mbps,
    settings.is_admin
      ? (form.server_upload_mbps ?? null)
      : settings.server_upload_mbps,
  );
  const recommendedProfile = recommendedId
    ? settings.profiles.find(p => p.id === recommendedId)
    : null;
  const downloadMbpsInvalid =
    validationMessage === DOWNLOAD_SPEED_VALIDATION_MESSAGE &&
    isDownloadSpeedOutOfRange(form);
  const serverUploadMbpsInvalid =
    validationMessage === SERVER_UPLOAD_VALIDATION_MESSAGE &&
    settings.is_admin &&
    isServerUploadOutOfRange(form);

  return (
    <form
      onSubmit={handleSubmit}
      noValidate
      className="max-w-5xl space-y-6"
    >
      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Gauge}
          title="Streaming & bandwidth"
          description="Tell Igloo about your connection so it can pick the right stream quality for you."
        />
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Your network speed"
            description="Used to recommend a stream profile for your viewing."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={downloadMbpsId}>Download speed (Mbps)</Label>
              <Input
                id={downloadMbpsId}
                name="download_mbps"
                type="number"
                inputMode="decimal"
                min={0.1}
                step={0.1}
                value={form.download_mbps ?? ""}
                onChange={event => handleDownloadMbpsChange(event.target.value)}
                disabled={updateMutation.isPending}
                aria-invalid={downloadMbpsInvalid ? "true" : undefined}
                aria-describedby={
                  downloadMbpsInvalid
                    ? `${downloadMbpsId}-description ${statusId}`
                    : `${downloadMbpsId}-description`
                }
                className={SETTINGS_INPUT_CLASS}
              />
              <p
                id={`${downloadMbpsId}-description`}
                className="text-sm text-muted-foreground"
              >
                Leave blank if unsure.
              </p>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Server upload cap"
            description="The home server's outbound bandwidth limit, applied when you stream from outside the home network."
          >
            {settings.is_admin ? (
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
            ) : (
              <>
                <p className="text-sm text-foreground">
                  {settings.server_upload_mbps != null
                    ? `${settings.server_upload_mbps} Mbps`
                    : "Not set (uncapped)"}
                </p>
                <p className="mt-2 text-sm text-muted-foreground">
                  Set by the server administrator. Affects your recommendation
                  when streaming from outside the home network.
                </p>
              </>
            )}
          </PlaybackSection>
          <PlaybackSection
            title="Recommended profile"
            titleId={recommendationTitleId}
            description="Calculated from your download speed and the server upload cap, with 20% headroom for audio and overhead."
          >
            <p
              id={recommendationId}
              role="region"
              aria-labelledby={recommendationTitleId}
              aria-live="polite"
              className="text-sm text-foreground"
            >
              {recommendedProfile
                ? `Recommended: ${recommendedProfile.label}`
                : "Enter your download speed to see a recommendation."}
            </p>
          </PlaybackSection>
        </CardContent>
      </Card>

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Sliders}
          title="Stream defaults"
          description="Defaults applied when you start a stream. You can still change each per video."
        />
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Preferred profile"
            description="The default profile when you start a stream."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={preferredProfileId}>Profile</Label>
              {/*
                Radix renders a visually-hidden aria-hidden <select name> for each
                of these for native form integration; they're never submitted (this
                form is JS-controlled). Chrome's Issues panel flags them "no label" —
                a false-positive on aria-hidden fields, not fixable without ejecting
                Radix. Keep the names so the fields stay identified.
              */}
              <Select
                name="preferred_profile"
                value={form.preferred_profile ?? NO_SELECTION_VALUE}
                onValueChange={handleProfileChange}
                disabled={updateMutation.isPending}
              >
                <SelectTrigger
                  id={preferredProfileId}
                  className={SETTINGS_SELECT_TRIGGER_CLASS}
                >
                  <SelectValue placeholder="Use recommended" />
                </SelectTrigger>
                <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                  <SelectItem
                    value={NO_SELECTION_VALUE}
                    className={SETTINGS_SELECT_ITEM_CLASS}
                  >
                    Use recommended
                  </SelectItem>
                  {settings.profiles.map(profile => (
                    <SelectItem
                      key={profile.id}
                      value={profile.id}
                      className={SETTINGS_SELECT_ITEM_CLASS}
                    >
                      {profile.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Preferred audio language"
            description="Igloo will pick the matching audio track when a movie has one."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={preferredAudioLanguageId}>Audio language</Label>
              <Select
                name="preferred_audio_language"
                value={form.preferred_audio_language ?? NO_SELECTION_VALUE}
                onValueChange={handleAudioLanguageChange}
                disabled={updateMutation.isPending}
              >
                <SelectTrigger
                  id={preferredAudioLanguageId}
                  className={SETTINGS_SELECT_TRIGGER_CLASS}
                >
                  <SelectValue placeholder="No preference" />
                </SelectTrigger>
                <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                  <SelectItem
                    value={NO_SELECTION_VALUE}
                    className={SETTINGS_SELECT_ITEM_CLASS}
                  >
                    No preference
                  </SelectItem>
                  {SORTED_LANGUAGE_ENTRIES.map(([code, name]) => (
                    <SelectItem
                      key={code}
                      value={code}
                      className={SETTINGS_SELECT_ITEM_CLASS}
                    >
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Preferred subtitles"
            description="Pick a default subtitle language, or always start with subtitles off."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={preferredSubtitleLanguageId}>
                Subtitle language
              </Label>
              <Select
                name="preferred_subtitle_language"
                value={form.preferred_subtitle_language ?? NO_SELECTION_VALUE}
                onValueChange={handleSubtitleLanguageChange}
                disabled={updateMutation.isPending}
              >
                <SelectTrigger
                  id={preferredSubtitleLanguageId}
                  className={SETTINGS_SELECT_TRIGGER_CLASS}
                >
                  <SelectValue placeholder="No preference" />
                </SelectTrigger>
                <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                  <SelectItem
                    value={NO_SELECTION_VALUE}
                    className={SETTINGS_SELECT_ITEM_CLASS}
                  >
                    No preference
                  </SelectItem>
                  <SelectItem
                    value={SUBTITLE_OFF_VALUE}
                    className={SETTINGS_SELECT_ITEM_CLASS}
                  >
                    Always off
                  </SelectItem>
                  {SORTED_LANGUAGE_ENTRIES.map(([code, name]) => (
                    <SelectItem
                      key={code}
                      value={code}
                      className={SETTINGS_SELECT_ITEM_CLASS}
                    >
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </PlaybackSection>
        </CardContent>
      </Card>

      {settings.is_admin && (
        <Card
          className={cn(
            SETTINGS_CARD_SURFACE_CLASS,
            MOTION_SETTINGS_SURFACE_CLASS,
          )}
        >
          <SettingsCardHeader
            icon={MonitorCog}
            title="Transcoding"
            description="Choose the hardware acceleration mode used for new transcodes."
          />
          <CardContent>
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
          </CardContent>
        </Card>
      )}

      <SettingsSaveBar
        title="Playback settings"
        statusId={statusId}
        statusMessage={
          validationMessage || "Saved preferences apply to your future streams."
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

type PlaybackSectionProps = {
  title: string;
  titleId?: string;
  description: string;
  children: ReactNode;
};

function PlaybackSection({
  title,
  titleId,
  description,
  children,
}: PlaybackSectionProps) {
  return (
    <section className="py-5 first:pt-0 last:pb-0">
      <h3 id={titleId} className="text-sm font-semibold text-foreground">
        {title}
      </h3>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      <div className="mt-3">{children}</div>
    </section>
  );
}
