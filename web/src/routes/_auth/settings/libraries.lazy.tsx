import { createLazyFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Fragment, useId, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  AlertCircle,
  Disc3,
  Film,
  FolderOpen,
  Library,
  Music,
  Scan,
  Trash2,
  Tv,
  User,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import SettingsCardHeader from "@/components/SettingsCardHeader";
import SettingsErrorCard from "@/components/SettingsErrorCard";
import SettingsLoadingCard from "@/components/SettingsLoadingCard";
import SettingsSaveBar from "@/components/SettingsSaveBar";
import { musicStatsQueryOpts, moviesStatsQueryOpts, settingsQueryOpts } from "@/lib/query-opts";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";
import { triggerMusicScan, triggerMovieScan, updateLibrarySettings } from "@/lib/api";
import { invalidateMovieLibraryQueries } from "@/lib/movie-library-cache";
import {
  ALBUM_DETAILS_KEY,
  ALBUMS_PAGINATED_KEY,
  LATEST_ALBUMS_KEY,
  LIKED_TRACK_IDS_KEY,
  LIKED_TRACKS_KEY,
  MUSIC_STATS_KEY,
  MUSICIAN_DETAILS_KEY,
  MUSICIANS_PAGINATED_KEY,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_TRACKS_KEY,
  PLAYLISTS_KEY,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
  SETTINGS_KEY,
  TRACKS_INFINITE_KEY,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { ApiResponseType, SettingsType } from "@/types";

export const Route = createLazyFileRoute("/_auth/settings/libraries")({
  component: LibrariesSettings,
});

type LibraryPathField = keyof SettingsType;
type ImplementedScan = "movies" | "music";

type LibrariesForm = {
  movies_dir: string;
  shows_dir: string;
  music_dir: string;
};

type LibrarySectionConfig = {
  field: LibraryPathField;
  scan: ImplementedScan | null;
  title: string;
  description: string;
  pathLabel: string;
  placeholder: string;
  Icon: LucideIcon;
  iconClassName: string;
  iconBackgroundClassName: string;
  scanButtonClassName: string;
  clearLabel: string;
};

type FormFeedback = {
  message: string;
  tone: "neutral" | "success" | "error";
};

type SettingsQueryData = ApiResponseType<SettingsType>;

const DEFAULT_FEEDBACK: FormFeedback = {
  message: "Saved library paths are used by scan and playback features.",
  tone: "neutral",
};

const LIBRARY_SECTIONS: LibrarySectionConfig[] = [
  {
    field: "movies_dir",
    scan: "movies",
    title: "Movies Library",
    description: "Manage your movie collection",
    pathLabel: "Movies library path",
    placeholder: "/srv/media/movies",
    Icon: Film,
    iconClassName: "text-primary",
    iconBackgroundClassName: "bg-primary/10",
    scanButtonClassName:
      "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground",
    clearLabel: "Clear movies library path",
  },
  {
    field: "shows_dir",
    scan: null,
    title: "TV Shows Library",
    description: "Manage your TV show collection",
    pathLabel: "TV shows library path",
    placeholder: "/srv/media/shows",
    Icon: Tv,
    iconClassName: "text-accent-teal",
    iconBackgroundClassName: "bg-accent-teal/10",
    scanButtonClassName:
      "bg-accent-teal text-accent-teal-foreground hover:bg-accent-teal/90 hover:text-accent-teal-foreground",
    clearLabel: "Clear TV shows library path",
  },
  {
    field: "music_dir",
    scan: "music",
    title: "Music Library",
    description: "Manage your music collection",
    pathLabel: "Music library path",
    placeholder: "/srv/media/music",
    Icon: Music,
    iconClassName: "text-primary",
    iconBackgroundClassName: "bg-primary/10",
    scanButtonClassName:
      "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground",
    clearLabel: "Clear music library path",
  },
];

function formFromSettings(settings: SettingsType): LibrariesForm {
  return {
    movies_dir: settings.movies_dir ?? "",
    shows_dir: settings.shows_dir ?? "",
    music_dir: settings.music_dir ?? "",
  };
}

function payloadFromForm(form: LibrariesForm): SettingsType {
  return {
    movies_dir: optionalLibraryPath(form.movies_dir),
    shows_dir: optionalLibraryPath(form.shows_dir),
    music_dir: optionalLibraryPath(form.music_dir),
  };
}

function optionalLibraryPath(value: string) {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function formHasChanges(form: LibrariesForm, settings: SettingsType) {
  const savedForm = formFromSettings(settings);
  return (
    form.movies_dir !== savedForm.movies_dir ||
    form.shows_dir !== savedForm.shows_dir ||
    form.music_dir !== savedForm.music_dir
  );
}

function fieldFromLibraryError(message?: string): LibraryPathField | null {
  const normalized = message?.toLowerCase() ?? "";
  if (normalized.includes("movies")) return "movies_dir";
  if (normalized.includes("shows")) return "shows_dir";
  if (normalized.includes("music")) return "music_dir";
  return null;
}

function scanLabel(scan: ImplementedScan) {
  return scan === "movies" ? "movies" : "music";
}

function LibrariesSettings() {
  const { data, isLoading } = useQuery(settingsQueryOpts());

  if (isLoading) {
    return <SettingsLoadingCard label="Loading library settings..." />;
  }

  if (data?.error) {
    return (
      <SettingsErrorCard
        title="Library settings unavailable"
        message={data.message || "Failed to load library settings."}
      />
    );
  }

  if (data?.error === false && data.data) {
    return <LibrariesSettingsForm settings={data.data} />;
  }

  return null;
}

type LibrariesSettingsFormProps = {
  settings: SettingsType;
};

function LibrariesSettingsForm({ settings }: LibrariesSettingsFormProps) {
  const queryClient = useQueryClient();
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [form, setForm] = useState<LibrariesForm>(() =>
    formFromSettings(settings),
  );
  const [feedback, setFeedback] = useState<FormFeedback>(DEFAULT_FEEDBACK);
  const [validationField, setValidationField] =
    useState<LibraryPathField | null>(null);
  const [activeScan, setActiveScan] = useState<ImplementedScan | null>(null);
  const formStatusId = useId();
  const hasChanges = formHasChanges(form, syncedSettings);

  if (settings !== syncedSettings) {
    const formIsClean = !formHasChanges(form, syncedSettings);
    setSyncedSettings(settings);
    if (formIsClean) {
      setForm(formFromSettings(settings));
      setValidationField(null);
    }
  }

  const updateMutation = useMutation({
    mutationFn: updateLibrarySettings,
    onSuccess: res => {
      if (res.error) {
        const message = res.message || "Failed to save library paths.";
        setValidationField(fieldFromLibraryError(message));
        setFeedback({ message, tone: "error" });
        showActionFailed("save library paths", message);
        return;
      }

      const nextSettings = res.data.settings;
      setSyncedSettings(nextSettings);
      setForm(formFromSettings(nextSettings));
      setValidationField(null);
      setFeedback({ message: "Library paths saved.", tone: "success" });
      queryClient.setQueryData<SettingsQueryData>([SETTINGS_KEY], {
        error: false,
        message: res.message,
        data: nextSettings,
      });
      queryClient.invalidateQueries({ queryKey: [SETTINGS_KEY] });
      showSuccess("Library paths saved");
    },
    onError: () => {
      setFeedback({
        message: "An unexpected error occurred while saving library paths.",
        tone: "error",
      });
      showActionFailed("save library paths", "An unexpected error occurred");
    },
  });

  const handlePathChange = (field: LibraryPathField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
    setValidationField(current => (current === field ? null : current));
    setFeedback({
      message: "Library path changes are ready to save.",
      tone: "neutral",
    });
  };

  const handleClearPath = (field: LibraryPathField) => {
    setForm(current => ({ ...current, [field]: "" }));
    setValidationField(current => (current === field ? null : current));
    setFeedback({
      message: "Library path changes are ready to save.",
      tone: "neutral",
    });
  };

  const handleReset = () => {
    setForm(formFromSettings(syncedSettings));
    setValidationField(null);
    setFeedback(DEFAULT_FEEDBACK);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!hasChanges) {
      setFeedback({
        message: "No library path changes to save.",
        tone: "neutral",
      });
      return;
    }

    setFeedback({ message: "Saving library paths...", tone: "neutral" });
    updateMutation.mutate(payloadFromForm(form));
  };

  const handleScan = async (scan: ImplementedScan) => {
    const field = scan === "movies" ? "movies_dir" : "music_dir";
    const savedPath = syncedSettings[field];
    const label = scanLabel(scan);
    if (!savedPath) {
      const message = `Save a ${label} library path before scanning.`;
      setFeedback({ message, tone: "error" });
      showActionFailed(`scan ${label} library`, message);
      return;
    }

    setActiveScan(scan);
    setFeedback({
      message: `Starting ${label} library scan...`,
      tone: "neutral",
    });

    try {
      const res =
        scan === "movies" ? await triggerMovieScan() : await triggerMusicScan();
      if (res.error) {
        const message = res.message || `Failed to start ${label} scan.`;
        setFeedback({ message, tone: "error" });
        showActionFailed(`scan ${label} library`, message);
        setActiveScan(current => (current === scan ? null : current));
        return;
      }

      setFeedback({
        message: `${label === "movies" ? "Movies" : "Music"} library scan started.`,
        tone: "success",
      });
      showSuccess(
        "Scan started",
        `${label === "movies" ? "Movies" : "Music"} library scan has been initiated`,
      );
      invalidateScanQueries(queryClient, scan);
      setActiveScan(current => (current === scan ? null : current));
    } catch {
      const message = `Failed to start ${label} scan.`;
      setFeedback({ message, tone: "error" });
      showActionFailed(`scan ${label} library`, message);
      setActiveScan(current => (current === scan ? null : current));
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      noValidate
      className="max-w-5xl space-y-6"
    >
      <Card className={SETTINGS_CARD_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={Library}
          title="Library Management"
          description="Manage your media library paths and scanning."
        />
        <CardContent className="space-y-6">
          {LIBRARY_SECTIONS.map((section, index) => (
            <Fragment key={section.field}>
              {index > 0 && <Separator className="bg-accent/50" />}
              <LibraryPathSection
                config={section}
                pathValue={form[section.field]}
                savedPath={syncedSettings[section.field]}
                invalid={validationField === section.field}
                disabled={updateMutation.isPending}
                scanPending={activeScan === section.scan}
                scanDisabled={
                  updateMutation.isPending ||
                  activeScan !== null ||
                  form[section.field] !==
                    formFromSettings(syncedSettings)[section.field]
                }
                formStatusId={formStatusId}
                onPathChange={value => handlePathChange(section.field, value)}
                onClearPath={() => handleClearPath(section.field)}
                onScan={
                  section.scan ? () => handleScan(section.scan as ImplementedScan) : undefined
                }
              >
                {section.field === "movies_dir" && (
                  <MoviesLibraryStats hasLibrary={Boolean(syncedSettings.movies_dir)} />
                )}
                {section.field === "shows_dir" && (
                  <TVShowsUnavailableStatus hasLibrary={Boolean(syncedSettings.shows_dir)} />
                )}
                {section.field === "music_dir" && (
                  <MusicLibraryStats hasLibrary={Boolean(syncedSettings.music_dir)} />
                )}
              </LibraryPathSection>
            </Fragment>
          ))}
        </CardContent>
      </Card>

      <SettingsSaveBar
        title="Library path settings"
        statusId={formStatusId}
        statusMessage={feedback.message}
        statusTone={feedback.tone}
        onReset={handleReset}
        resetLabel="Reset library paths"
        resetDisabled={!hasChanges || updateMutation.isPending}
        saveLabel="Save library paths"
        saveDisabled={!hasChanges || updateMutation.isPending}
        isPending={updateMutation.isPending}
        className="bg-card/70"
      />
    </form>
  );
}

type LibraryPathSectionProps = {
  config: LibrarySectionConfig;
  pathValue: string;
  savedPath: string | null;
  invalid: boolean;
  disabled: boolean;
  scanPending: boolean;
  scanDisabled: boolean;
  formStatusId: string;
  children: ReactNode;
  onPathChange: (value: string) => void;
  onClearPath: () => void;
  onScan?: () => void;
};

function LibraryPathSection({
  config,
  pathValue,
  savedPath,
  invalid,
  disabled,
  scanPending,
  scanDisabled,
  formStatusId,
  children,
  onPathChange,
  onClearPath,
  onScan,
}: LibraryPathSectionProps) {
  const headingId = useId();
  const pathInputId = useId();
  const descriptionId = `${pathInputId}-description`;
  const pathStatusId = `${pathInputId}-status`;
  const Icon = config.Icon;
  const trimmedPath = pathValue.trim();
  const savedValue = savedPath ?? "";
  const hasSavedPath = savedValue !== "";
  const hasPathValue = trimmedPath !== "";
  const pathChanged = pathValue !== savedValue;

  return (
    <section aria-labelledby={headingId} className="space-y-4">
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex size-10 shrink-0 items-center justify-center rounded-lg",
            config.iconBackgroundClassName,
          )}
          aria-hidden="true"
        >
          <Icon className={cn("size-5", config.iconClassName)} aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <h3 id={headingId} className="text-lg font-semibold text-foreground">
            {config.title}
          </h3>
          <p className="text-sm text-muted-foreground">{config.description}</p>
        </div>
      </div>

      <div className="space-y-3 rounded-lg border border-border/50 bg-card/50 p-4">
        <div
          id={pathStatusId}
          className={cn(
            "flex items-center gap-3 text-sm",
            invalid ? "text-destructive" : "text-muted-foreground",
          )}
        >
          {hasSavedPath ? (
            <FolderOpen className="size-5 shrink-0" aria-hidden="true" />
          ) : (
            <AlertCircle className="size-5 shrink-0" aria-hidden="true" />
          )}
          <p>{libraryPathStatusText(hasSavedPath, hasPathValue, pathChanged)}</p>
        </div>
        <div className="grid gap-2">
          <Label htmlFor={pathInputId}>{config.pathLabel}</Label>
          <Input
            id={pathInputId}
            type="text"
            value={pathValue}
            onChange={event => onPathChange(event.target.value)}
            placeholder={config.placeholder}
            disabled={disabled}
            aria-invalid={invalid || undefined}
            aria-describedby={`${descriptionId} ${pathStatusId} ${formStatusId}`}
            autoComplete="off"
            className={SETTINGS_INPUT_CLASS}
          />
          <p id={descriptionId} className="text-sm text-muted-foreground">
            Enter a directory path readable by the Igloo server. Leave blank to
            clear this library path.
          </p>
        </div>
        {hasPathValue && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onClearPath}
            disabled={disabled}
            aria-label={config.clearLabel}
            className="w-full text-muted-foreground hover:bg-muted hover:text-destructive sm:w-fit"
          >
            <Trash2 className="size-4" aria-hidden="true" />
            Clear path
          </Button>
        )}
      </div>

      {children}

      {config.scan && hasSavedPath && (
        <Button
          type="button"
          onClick={onScan}
          disabled={scanDisabled}
          aria-busy={scanPending}
          aria-label={
            scanPending
              ? `Scanning ${scanLabel(config.scan)} library, please wait`
              : `Scan ${scanLabel(config.scan)} library`
          }
          className={cn(
            "w-full disabled:opacity-50",
            config.scanButtonClassName,
          )}
        >
          {scanPending ? (
            <>
              <Spinner className="size-4" aria-hidden="true" />
              <span>Scanning...</span>
            </>
          ) : (
            <>
              <Scan className="size-4" aria-hidden="true" />
              Scan Library
            </>
          )}
        </Button>
      )}
    </section>
  );
}

function libraryPathStatusText(
  hasSavedPath: boolean,
  hasPathValue: boolean,
  pathChanged: boolean,
) {
  if (pathChanged && !hasPathValue) return "Path will be cleared after saving.";
  if (pathChanged && hasSavedPath) return "Path changed, save to apply.";
  if (pathChanged) return "Path ready to save.";
  if (hasSavedPath) return "Library path configured.";
  return "No library path configured.";
}

type StatsProps = {
  hasLibrary: boolean;
};

function MoviesLibraryStats({ hasLibrary }: StatsProps) {
  const { data, isLoading } = useQuery({
    ...moviesStatsQueryOpts(),
    enabled: hasLibrary,
  });
  const stats = data?.error === false ? data.data : null;

  if (!hasLibrary) return null;

  return (
    <div className="grid gap-4" aria-label="Movies library statistics">
      <StatItem
        label="Movies"
        value={stats?.total_movies ?? 0}
        loading={isLoading}
        loadingLabel="Loading movies count"
        icon={<Film className="size-5 text-primary" aria-hidden="true" />}
        iconBackgroundClassName="bg-primary/10"
      />
    </div>
  );
}

function MusicLibraryStats({ hasLibrary }: StatsProps) {
  const { data, isLoading } = useQuery({
    ...musicStatsQueryOpts(),
    enabled: hasLibrary,
  });
  const stats = data?.error === false ? data.data : null;

  if (!hasLibrary) return null;

  return (
    <div
      className="grid gap-4 sm:grid-cols-3"
      aria-label="Music library statistics"
    >
      <StatItem
        label="Albums"
        value={stats?.total_albums ?? 0}
        loading={isLoading}
        loadingLabel="Loading albums count"
        icon={<Disc3 className="size-5 text-primary" aria-hidden="true" />}
        iconBackgroundClassName="bg-primary/10"
      />
      <StatItem
        label="Tracks"
        value={stats?.total_tracks ?? 0}
        loading={isLoading}
        loadingLabel="Loading tracks count"
        icon={<Music className="size-5 text-primary" aria-hidden="true" />}
        iconBackgroundClassName="bg-primary/10"
      />
      <StatItem
        label="Musicians"
        value={stats?.total_musicians ?? 0}
        loading={isLoading}
        loadingLabel="Loading musicians count"
        icon={<User className="size-5 text-primary" aria-hidden="true" />}
        iconBackgroundClassName="bg-primary/10"
      />
    </div>
  );
}

type StatItemProps = {
  label: string;
  value: number;
  loading: boolean;
  loadingLabel: string;
  icon: ReactNode;
  iconBackgroundClassName: string;
};

function StatItem({
  label,
  value,
  loading,
  loadingLabel,
  icon,
  iconBackgroundClassName,
}: StatItemProps) {
  return (
    <div className="rounded-lg border border-border/50 bg-card/50 p-4">
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex size-10 shrink-0 items-center justify-center rounded-lg",
            iconBackgroundClassName,
          )}
          aria-hidden="true"
        >
          {icon}
        </div>
        {loading ? (
          <div role="status" className="flex items-center gap-2 text-muted-foreground">
            <Spinner className="size-5 text-primary" aria-hidden="true" />
            <span className="sr-only">{loadingLabel}</span>
          </div>
        ) : (
          <div>
            <p className="text-2xl font-bold text-foreground">
              {value.toLocaleString()}
            </p>
            <p className="text-sm text-muted-foreground">{label}</p>
          </div>
        )}
      </div>
    </div>
  );
}

function TVShowsUnavailableStatus({ hasLibrary }: StatsProps) {
  if (!hasLibrary) {
    return (
      <div className="rounded-lg border border-dashed border-border bg-card/30 p-4 text-sm text-muted-foreground">
        TV shows can be configured now. Scanning will be available after TV show
        support is implemented.
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-dashed border-border bg-card/30 p-6">
      <div className="flex items-center gap-3 text-muted-foreground">
        <AlertCircle className="size-5 shrink-0" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-sm font-medium text-foreground">
            TV shows scanning unavailable
          </p>
          <p className="text-sm">
            The TV shows path is saved, but TV scanning is not implemented yet.
          </p>
        </div>
      </div>
    </div>
  );
}

function invalidateScanQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  scan: ImplementedScan,
) {
  if (scan === "movies") {
    invalidateMovieLibraryQueries(queryClient);
    return;
  }

  const queryKeys = [
    MUSIC_STATS_KEY,
    LATEST_ALBUMS_KEY,
    ALBUMS_PAGINATED_KEY,
    ALBUM_DETAILS_KEY,
    MUSICIANS_PAGINATED_KEY,
    MUSICIAN_DETAILS_KEY,
    TRACKS_INFINITE_KEY,
    LIKED_TRACKS_KEY,
    LIKED_TRACK_IDS_KEY,
    PLAYLISTS_KEY,
    PLAYLIST_DETAILS_KEY,
    PLAYLIST_TRACKS_KEY,
  ];

  queryKeys.forEach(key => {
    queryClient.invalidateQueries({ queryKey: [key] });
  });
}
