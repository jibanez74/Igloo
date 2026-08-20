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
  Server,
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
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import {
  DOWNLOAD_SPEED_MAX_MBPS,
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
import { useDevicePlaybackPreferences } from "@/hooks/useDevicePlaybackPreferences";
import { setDevicePlaybackPreferences } from "@/lib/playback-preferences";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import { authUserQueryOpts, playbackSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  DevicePlaybackPreferences,
  HardwareAccelerationDevice,
  PlaybackSettingsResponseType,
  PlaybackSettingsType,
  UpdatePlaybackSettingsRequest,
} from "@/types";

export const Route = createFileRoute("/_auth/settings/playback")({
  component: PlaybackSettings,
});

const NO_SELECTION_VALUE = "__none__";
const SERVER_UPLOAD_MAX_MBPS = 100_000;
const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  `Download speed must be between 0 and ${DOWNLOAD_SPEED_MAX_MBPS} Mbps.`;
const SERVER_UPLOAD_VALIDATION_MESSAGE =
  `Server upload bandwidth must be greater than 0 and less than ${SERVER_UPLOAD_MAX_MBPS} Mbps.`;
/** Explains that the device card writes to this browser rather than the account. */
const DEVICE_SCOPE_DESCRIPTION =
  "Saved in this browser only. Your other devices keep their own settings.";
const DEVICE_SAVED_SUFFIX = "Saved on this device.";

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

function parseMbpsInput(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === "") return null;
  const parsed = Number.parseFloat(trimmed);
  return Number.isFinite(parsed) ? parsed : null;
}

function isDownloadSpeedOutOfRange(value: number | null) {
  return value != null && (value <= 0 || value >= DOWNLOAD_SPEED_MAX_MBPS);
}

function isServerUploadOutOfRange(form: UpdatePlaybackSettingsRequest) {
  return (
    form.server_upload_mbps != null &&
    (form.server_upload_mbps <= 0 ||
      form.server_upload_mbps >= SERVER_UPLOAD_MAX_MBPS)
  );
}

function languageLabel(code: string | null) {
  if (code === null) return "No preference";
  if (code === SUBTITLE_OFF_VALUE) return "Always off";
  return LANGUAGE_NAMES[code] ?? code;
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

type DevicePlaybackCardsProps = {
  settings: PlaybackSettingsType;
  userId: number;
  isAdmin: boolean;
};

/**
 * The per-device half of the page. These preferences live in localStorage and
 * apply the moment they change, so there is no save bar -- the confirmation is
 * the live announcement instead.
 */
function DevicePlaybackCards({
  settings,
  userId,
  isAdmin,
}: DevicePlaybackCardsProps) {
  const downloadMbpsId = useId();
  const preferredProfileId = useId();
  const recommendationId = useId();
  const recommendationTitleId = useId();
  const deviceStatusId = useId();
  const preferredAudioLanguageId = useId();
  const preferredSubtitleLanguageId = useId();

  const prefs = useDevicePlaybackPreferences(userId);
  // The input keeps its own text so a half-typed "0." neither reaches storage
  // nor snaps back under the user; committed values flow from `prefs`.
  const [downloadText, setDownloadText] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState("");
  const [announcementKey, setAnnouncementKey] = useState(0);

  const announce = (message: string) => {
    setAnnouncement(`${message} ${DEVICE_SAVED_SUFFIX}`);
    setAnnouncementKey(current => current + 1);
  };

  const persist = (
    patch: Partial<DevicePlaybackPreferences>,
    message: string,
  ) => {
    setDevicePlaybackPreferences(userId, patch);
    announce(message);
  };

  const pendingDownload =
    downloadText === null ? prefs.downloadMbps : parseMbpsInput(downloadText);
  const downloadInvalid = isDownloadSpeedOutOfRange(pendingDownload);

  const handleDownloadMbpsChange = (value: string) => {
    setDownloadText(value);
    const parsed = parseMbpsInput(value);
    if (isDownloadSpeedOutOfRange(parsed)) return;
    if (parsed === prefs.downloadMbps) return;

    setDevicePlaybackPreferences(userId, { downloadMbps: parsed });
  };

  const handleDownloadMbpsCommit = () => {
    if (downloadInvalid) return;
    announce(
      pendingDownload === null
        ? "Download speed cleared."
        : `Download speed set to ${pendingDownload} Mbps.`,
    );
  };

  const handleProfileChange = (value: string) => {
    const preferredProfile = value === NO_SELECTION_VALUE ? null : value;
    const label = preferredProfile
      ? (settings.profiles.find(p => p.id === preferredProfile)?.label ??
        preferredProfile)
      : "Use recommended";
    persist({ preferredProfile }, `Preferred profile set to ${label}.`);
  };

  const handleAudioLanguageChange = (value: string) => {
    const preferredAudioLanguage =
      value === NO_SELECTION_VALUE ? null : value;
    persist(
      { preferredAudioLanguage },
      `Preferred audio language set to ${languageLabel(preferredAudioLanguage)}.`,
    );
  };

  const handleSubtitleLanguageChange = (value: string) => {
    const preferredSubtitleLanguage =
      value === NO_SELECTION_VALUE ? null : value;
    persist(
      { preferredSubtitleLanguage },
      `Preferred subtitles set to ${languageLabel(preferredSubtitleLanguage)}.`,
    );
  };

  const recommendedId = recommendedProfileId(
    settings.profiles,
    downloadInvalid ? prefs.downloadMbps : pendingDownload,
    settings.server_upload_mbps,
  );
  const recommendedProfile = recommendedId
    ? settings.profiles.find(p => p.id === recommendedId)
    : null;

  return (
    <>
      <LiveAnnouncer message={announcement} announcementKey={announcementKey} />
      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Gauge}
          title="Streaming & bandwidth"
          description={`Tell Igloo about this connection so it can pick the right stream quality. ${DEVICE_SCOPE_DESCRIPTION}`}
        />
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Your network speed"
            description="Used to recommend a stream profile for this device."
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
                value={downloadText ?? prefs.downloadMbps ?? ""}
                onChange={event => handleDownloadMbpsChange(event.target.value)}
                onBlur={handleDownloadMbpsCommit}
                aria-invalid={downloadInvalid ? "true" : undefined}
                aria-describedby={
                  downloadInvalid
                    ? `${downloadMbpsId}-description ${deviceStatusId}`
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
              <p
                id={deviceStatusId}
                aria-live="polite"
                className="text-sm text-destructive empty:hidden"
              >
                {downloadInvalid ? DOWNLOAD_SPEED_VALIDATION_MESSAGE : ""}
              </p>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Server upload cap"
            description="The home server's outbound bandwidth limit, applied when you stream from outside the home network."
          >
            <p className="text-sm text-foreground">
              {settings.server_upload_mbps != null
                ? `${settings.server_upload_mbps} Mbps`
                : "Not set (uncapped)"}
            </p>
            <p className="mt-2 text-sm text-muted-foreground">
              {isAdmin
                ? "Set for the whole server below. Affects your recommendation when streaming from outside the home network."
                : "Set by the server administrator. Affects your recommendation when streaming from outside the home network."}
            </p>
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
          description={`Defaults applied when you start a stream; you can still change each per video. ${DEVICE_SCOPE_DESCRIPTION}`}
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
                value={prefs.preferredProfile ?? NO_SELECTION_VALUE}
                onValueChange={handleProfileChange}
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
                value={prefs.preferredAudioLanguage ?? NO_SELECTION_VALUE}
                onValueChange={handleAudioLanguageChange}
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
                value={prefs.preferredSubtitleLanguage ?? NO_SELECTION_VALUE}
                onValueChange={handleSubtitleLanguageChange}
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
    </>
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
