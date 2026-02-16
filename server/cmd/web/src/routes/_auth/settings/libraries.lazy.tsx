import { createLazyFileRoute } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useTransition } from "react";
import {
  Library,
  Music,
  Film,
  Tv,
  FolderOpen,
  Scan,
  Plus,
  Trash2,
  Disc3,
  User,
  AlertCircle,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import { musicStatsQueryOpts, settingsQueryOpts } from "@/lib/query-opts";
import { showError, showSuccess, showActionFailed } from "@/lib/toast-helpers";
import { triggerMusicScan } from "@/lib/api";
import { useQueryClient } from "@tanstack/react-query";
import {
  MUSIC_STATS_KEY,
  ALBUMS_KEY,
  TRACKS_KEY,
  MUSICIANS_KEY,
  LATEST_ALBUMS_KEY,
  ALBUMS_PAGINATED_KEY,
  MUSICIANS_PAGINATED_KEY,
  TRACKS_INFINITE_KEY,
} from "@/lib/constants";

export const Route = createLazyFileRoute("/_auth/settings/libraries")({
  component: LibrariesSettings,
});

function LibrariesSettings() {
  return (
    <div className='space-y-8'>
      <Card className='border-slate-700/50 bg-slate-800/30'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-white'>
            <Library className='size-5 text-amber-400' aria-hidden='true' />
            Library Management
          </CardTitle>
          <CardDescription className='text-slate-300'>
            Manage your media library paths and scanning
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <MoviesLibrarySection />
          <Separator className='bg-slate-700/50' />
          <TVShowsLibrarySection />
          <Separator className='bg-slate-700/50' />
          <MusicLibrarySection />
        </CardContent>
      </Card>
    </div>
  );
}

function MusicLibrarySection() {
  const { data: statsData, isLoading: statsLoading } = useQuery(
    musicStatsQueryOpts(),
  );

  const { data: settingsData } = useQuery(settingsQueryOpts());
  const queryClient = useQueryClient();
  const [isScanning, startTransition] = useTransition();

  const stats = statsData?.error === false ? statsData.data : null;
  const settings = settingsData?.error === false ? settingsData.data : null;
  const libraryPath: string | null = settings?.music_dir ?? null;
  const hasLibrary = Boolean(libraryPath);

  const handleScan = () => {
    if (!libraryPath) {
      showError(
        "No library path",
        "Please configure a music library path first",
      );
      return;
    }

    startTransition(async () => {
      try {
        const res = await triggerMusicScan();
        if (res.error) {
          showActionFailed("scan music library", res.message);
        } else {
          showSuccess("Scan started", "Music library scan has been initiated");
          // Invalidate all music-related queries to get fresh data after scan
          const musicQueryKeys = [
            MUSIC_STATS_KEY,
            ALBUMS_KEY,
            TRACKS_KEY,
            MUSICIANS_KEY,
            LATEST_ALBUMS_KEY,
            ALBUMS_PAGINATED_KEY,
            MUSICIANS_PAGINATED_KEY,
            TRACKS_INFINITE_KEY,
          ];
          musicQueryKeys.forEach(key => {
            queryClient.invalidateQueries({ queryKey: [key] });
          });
        }
      } catch {
        showActionFailed("scan music library", "Failed to start scan");
      }
    });
  };

  const handleAddLibrary = () => {
    showError("Not implemented", "Adding libraries will be available soon");
  };

  const handleRemoveLibrary = () => {
    showError("Not implemented", "Removing libraries will be available soon");
  };

  const sectionId = "music-library-section";
  const headingId = "music-library-heading";
  const pathId = "music-library-path";
  const statsId = "music-library-stats";

  return (
    <section id={sectionId} aria-labelledby={headingId} className='space-y-4'>
      {/* Section Header */}
      <div className='flex items-center gap-3'>
        <div
          className='flex size-10 shrink-0 items-center justify-center rounded-lg bg-amber-500/10'
          aria-hidden='true'
        >
          <Music className='size-5 text-amber-400' aria-hidden='true' />
        </div>
        <div className='min-w-0'>
          <h3 id={headingId} className='text-lg font-semibold text-white'>
            Music Library
          </h3>
          <p className='text-sm text-slate-300'>Manage your music collection</p>
        </div>
      </div>

      {/* Library Path */}
      <div
        id={pathId}
        role='region'
        aria-labelledby={`${pathId}-label`}
        className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4'
      >
        {hasLibrary ? (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex min-w-0 flex-1 items-center gap-3'>
              <FolderOpen
                className='size-5 shrink-0 text-slate-300'
                aria-hidden='true'
              />
              <div className='min-w-0 flex-1'>
                <p
                  id={`${pathId}-label`}
                  className='text-sm font-medium text-slate-300'
                >
                  Library Path
                </p>
                <p
                  className='truncate text-sm text-slate-300'
                  title={libraryPath!}
                  aria-label={`Music library path: ${libraryPath}`}
                >
                  {libraryPath}
                </p>
              </div>
            </div>
            <div className='flex items-center gap-2 sm:shrink-0'>
              <Button
                variant='ghost'
                size='sm'
                onClick={handleRemoveLibrary}
                aria-label={`Remove music library path: ${libraryPath}`}
                className='text-slate-300 hover:bg-slate-800 hover:text-red-400'
              >
                <Trash2 className='mr-2 size-4' aria-hidden='true' />
                Remove
              </Button>
            </div>
          </div>
        ) : (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3 text-slate-300'>
              <AlertCircle className='size-5 shrink-0' aria-hidden='true' />
              <p id={`${pathId}-label`} className='text-sm'>
                No library path configured
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={handleAddLibrary}
              aria-label='Add music library path'
              className='border-slate-600 text-slate-300 hover:bg-slate-700 hover:text-white'
            >
              <Plus className='mr-2 size-4' aria-hidden='true' />
              Add Library
            </Button>
          </div>
        )}
      </div>

      {/* Stats - Only show if library is configured */}
      {hasLibrary && (
        <div
          id={statsId}
          role='region'
          aria-labelledby={`${statsId}-label`}
          className='grid gap-4 sm:grid-cols-3'
        >
          <p id={`${statsId}-label`} className='sr-only'>
            Music library statistics
          </p>
          <div
            role='group'
            tabIndex={0}
            aria-label={`Total albums: ${statsLoading ? "Loading" : (stats?.total_albums?.toLocaleString() ?? 0)}`}
            className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
          >
            <div className='flex items-center gap-3'>
              <div
                className='flex size-10 items-center justify-center rounded-lg bg-amber-500/10'
                aria-hidden='true'
              >
                <Disc3 className='size-5 text-amber-400' aria-hidden='true' />
              </div>
              <div>
                {statsLoading ? (
                  <>
                    <Spinner
                      className='size-5 text-amber-400'
                      aria-hidden='true'
                    />
                    <span className='sr-only'>Loading albums count</span>
                  </>
                ) : (
                  <>
                    <p
                      className='text-2xl font-bold text-white'
                      aria-live='polite'
                    >
                      {stats?.total_albums?.toLocaleString() ?? 0}
                    </p>
                    <p className='text-sm text-slate-300'>Albums</p>
                  </>
                )}
              </div>
            </div>
          </div>

          <div
            role='group'
            tabIndex={0}
            aria-label={`Total tracks: ${statsLoading ? "Loading" : (stats?.total_tracks?.toLocaleString() ?? 0)}`}
            className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
          >
            <div className='flex items-center gap-3'>
              <div
                className='flex size-10 items-center justify-center rounded-lg bg-amber-500/10'
                aria-hidden='true'
              >
                <Music className='size-5 text-amber-400' aria-hidden='true' />
              </div>
              <div>
                {statsLoading ? (
                  <>
                    <Spinner
                      className='size-5 text-amber-400'
                      aria-hidden='true'
                    />
                    <span className='sr-only'>Loading tracks count</span>
                  </>
                ) : (
                  <>
                    <p
                      className='text-2xl font-bold text-white'
                      aria-live='polite'
                    >
                      {stats?.total_tracks?.toLocaleString() ?? 0}
                    </p>
                    <p className='text-sm text-slate-300'>Tracks</p>
                  </>
                )}
              </div>
            </div>
          </div>

          <div
            role='group'
            tabIndex={0}
            aria-label={`Total musicians: ${statsLoading ? "Loading" : (stats?.total_musicians?.toLocaleString() ?? 0)}`}
            className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
          >
            <div className='flex items-center gap-3'>
              <div
                className='flex size-10 items-center justify-center rounded-lg bg-amber-500/10'
                aria-hidden='true'
              >
                <User className='size-5 text-amber-400' aria-hidden='true' />
              </div>
              <div>
                {statsLoading ? (
                  <>
                    <Spinner
                      className='size-5 text-amber-400'
                      aria-hidden='true'
                    />
                    <span className='sr-only'>Loading musicians count</span>
                  </>
                ) : (
                  <>
                    <p
                      className='text-2xl font-bold text-white'
                      aria-live='polite'
                    >
                      {stats?.total_musicians?.toLocaleString() ?? 0}
                    </p>
                    <p className='text-sm text-slate-300'>Musicians</p>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Scan Button - Only show if library is configured */}
      {hasLibrary && (
        <Button
          onClick={handleScan}
          disabled={isScanning}
          aria-label={
            isScanning
              ? "Scanning music library, please wait"
              : "Scan music library"
          }
          className='w-full bg-amber-500 text-slate-900 hover:bg-amber-400 hover:text-slate-900 disabled:opacity-50'
        >
          {isScanning ? (
            <>
              <Spinner className='mr-2 size-4' aria-hidden='true' />
              <span aria-live='polite'>Scanning...</span>
            </>
          ) : (
            <>
              <Scan className='mr-2 size-4' aria-hidden='true' />
              Scan Library
            </>
          )}
        </Button>
      )}
    </section>
  );
}

function MoviesLibrarySection() {
  const { data: settingsData } = useQuery(settingsQueryOpts());
  const [isScanning, startTransition] = useTransition();

  const settings = settingsData?.error === false ? settingsData.data : null;
  const libraryPath: string | null = settings?.movies_dir ?? null;
  const hasLibrary = Boolean(libraryPath);

  const handleScan = () => {
    startTransition(async () => {
      showError("Not implemented", "Library scanning will be available soon");
    });
  };

  const handleAddLibrary = () => {
    showError("Not implemented", "Adding libraries will be available soon");
  };

  const handleRemoveLibrary = () => {
    showError("Not implemented", "Removing libraries will be available soon");
  };

  const sectionId = "movies-library-section";
  const headingId = "movies-library-heading";
  const pathId = "movies-library-path";

  return (
    <section id={sectionId} aria-labelledby={headingId} className='space-y-4'>
      {/* Section Header */}
      <div className='flex items-center gap-3'>
        <div
          className='flex size-10 shrink-0 items-center justify-center rounded-lg bg-cyan-500/10'
          aria-hidden='true'
        >
          <Film className='size-5 text-cyan-400' aria-hidden='true' />
        </div>
        <div className='min-w-0'>
          <h3 id={headingId} className='text-lg font-semibold text-white'>
            Movies Library
          </h3>
          <p className='text-sm text-slate-300'>Manage your movie collection</p>
        </div>
      </div>

      {/* Library Path */}
      <div
        id={pathId}
        role='region'
        aria-labelledby={`${pathId}-label`}
        className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4'
      >
        {hasLibrary ? (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex min-w-0 flex-1 items-center gap-3'>
              <FolderOpen
                className='size-5 shrink-0 text-slate-300'
                aria-hidden='true'
              />
              <div className='min-w-0 flex-1'>
                <p
                  id={`${pathId}-label`}
                  className='text-sm font-medium text-slate-300'
                >
                  Library Path
                </p>
                <p
                  className='truncate text-sm text-slate-300'
                  title={libraryPath!}
                  aria-label={`Movies library path: ${libraryPath}`}
                >
                  {libraryPath}
                </p>
              </div>
            </div>
            <div className='flex items-center gap-2 sm:shrink-0'>
              <Button
                variant='ghost'
                size='sm'
                onClick={handleRemoveLibrary}
                aria-label={`Remove movies library path: ${libraryPath}`}
                className='text-slate-300 hover:bg-slate-800 hover:text-red-400'
              >
                <Trash2 className='mr-2 size-4' aria-hidden='true' />
                Remove
              </Button>
            </div>
          </div>
        ) : (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3 text-slate-300'>
              <AlertCircle className='size-5 shrink-0' aria-hidden='true' />
              <p id={`${pathId}-label`} className='text-sm'>
                No library path configured
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={handleAddLibrary}
              aria-label='Add movies library path'
              className='border-slate-600 text-slate-300 hover:bg-slate-700 hover:text-white'
            >
              <Plus className='mr-2 size-4' aria-hidden='true' />
              Add Library
            </Button>
          </div>
        )}
      </div>

      {/* Stats Placeholder - Only show if library is configured */}
      {hasLibrary && (
        <div
          role='region'
          tabIndex={0}
          aria-label='Movie library statistics: Not available yet'
          className='rounded-lg border border-dashed border-slate-700 bg-slate-900/30 p-6 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
        >
          <div className='flex items-center gap-3 text-slate-300'>
            <AlertCircle className='size-5' aria-hidden='true' />
            <p className='text-sm'>
              Movie statistics will be available after movies feature is
              implemented
            </p>
          </div>
        </div>
      )}

      {/* Scan Button - Only show if library is configured */}
      {hasLibrary && (
        <Button
          onClick={handleScan}
          disabled={isScanning}
          aria-label={
            isScanning
              ? "Scanning movies library, please wait"
              : "Scan movies library"
          }
          className='w-full bg-cyan-500 text-slate-900 hover:bg-cyan-400 hover:text-slate-900 disabled:opacity-50'
        >
          {isScanning ? (
            <>
              <Spinner className='mr-2 size-4' aria-hidden='true' />
              <span aria-live='polite'>Scanning...</span>
            </>
          ) : (
            <>
              <Scan className='mr-2 size-4' aria-hidden='true' />
              Scan Library
            </>
          )}
        </Button>
      )}
    </section>
  );
}

function TVShowsLibrarySection() {
  const { data: settingsData } = useQuery(settingsQueryOpts());
  const [isScanning, startTransition] = useTransition();

  const settings = settingsData?.error === false ? settingsData.data : null;
  const libraryPath: string | null = settings?.shows_dir ?? null;
  const hasLibrary = Boolean(libraryPath);

  const handleScan = () => {
    startTransition(async () => {
      showError("Not implemented", "Library scanning will be available soon");
    });
  };

  const handleAddLibrary = () => {
    showError("Not implemented", "Adding libraries will be available soon");
  };

  const handleRemoveLibrary = () => {
    showError("Not implemented", "Removing libraries will be available soon");
  };

  const sectionId = "tv-shows-library-section";
  const headingId = "tv-shows-library-heading";
  const pathId = "tv-shows-library-path";

  return (
    <section id={sectionId} aria-labelledby={headingId} className='space-y-4'>
      {/* Section Header */}
      <div className='flex items-center gap-3'>
        <div
          className='flex size-10 shrink-0 items-center justify-center rounded-lg bg-purple-500/10'
          aria-hidden='true'
        >
          <Tv className='size-5 text-purple-400' aria-hidden='true' />
        </div>
        <div className='min-w-0'>
          <h3 id={headingId} className='text-lg font-semibold text-white'>
            TV Shows Library
          </h3>
          <p className='text-sm text-slate-300'>
            Manage your TV show collection
          </p>
        </div>
      </div>

      {/* Library Path */}
      <div
        id={pathId}
        role='region'
        aria-labelledby={`${pathId}-label`}
        className='rounded-lg border border-slate-700/50 bg-slate-900/50 p-4'
      >
        {hasLibrary ? (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex min-w-0 flex-1 items-center gap-3'>
              <FolderOpen
                className='size-5 shrink-0 text-slate-300'
                aria-hidden='true'
              />
              <div className='min-w-0 flex-1'>
                <p
                  id={`${pathId}-label`}
                  className='text-sm font-medium text-slate-300'
                >
                  Library Path
                </p>
                <p
                  className='truncate text-sm text-slate-300'
                  title={libraryPath!}
                  aria-label={`TV shows library path: ${libraryPath}`}
                >
                  {libraryPath}
                </p>
              </div>
            </div>
            <div className='flex items-center gap-2 sm:shrink-0'>
              <Button
                variant='ghost'
                size='sm'
                onClick={handleRemoveLibrary}
                aria-label={`Remove TV shows library path: ${libraryPath}`}
                className='text-slate-300 hover:bg-slate-800 hover:text-red-400'
              >
                <Trash2 className='mr-2 size-4' aria-hidden='true' />
                Remove
              </Button>
            </div>
          </div>
        ) : (
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3 text-slate-300'>
              <AlertCircle className='size-5 shrink-0' aria-hidden='true' />
              <p id={`${pathId}-label`} className='text-sm'>
                No library path configured
              </p>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={handleAddLibrary}
              aria-label='Add TV shows library path'
              className='border-slate-600 text-slate-300 hover:bg-slate-700 hover:text-white'
            >
              <Plus className='mr-2 size-4' aria-hidden='true' />
              Add Library
            </Button>
          </div>
        )}
      </div>

      {/* Stats Placeholder - Only show if library is configured */}
      {hasLibrary && (
        <div
          role='region'
          tabIndex={0}
          aria-label='TV shows library statistics: Not available yet'
          className='rounded-lg border border-dashed border-slate-700 bg-slate-900/30 p-6 focus:ring-2 focus:ring-purple-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none'
        >
          <div className='flex items-center gap-3 text-slate-300'>
            <AlertCircle className='size-5' aria-hidden='true' />
            <p className='text-sm'>
              TV show statistics will be available after TV shows feature is
              implemented
            </p>
          </div>
        </div>
      )}

      {/* Scan Button - Only show if library is configured */}
      {hasLibrary && (
        <Button
          onClick={handleScan}
          disabled={isScanning}
          aria-label={
            isScanning
              ? "Scanning TV shows library, please wait"
              : "Scan TV shows library"
          }
          className='w-full bg-purple-500 text-slate-900 hover:bg-purple-400 hover:text-slate-900 disabled:opacity-50'
        >
          {isScanning ? (
            <>
              <Spinner className='mr-2 size-4' aria-hidden='true' />
              <span aria-live='polite'>Scanning...</span>
            </>
          ) : (
            <>
              <Scan className='mr-2 size-4' aria-hidden='true' />
              Scan Library
            </>
          )}
        </Button>
      )}
    </section>
  );
}
