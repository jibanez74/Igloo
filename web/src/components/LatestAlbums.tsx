import { useQuery } from "@tanstack/react-query";
import { AlertCircle, Music } from "lucide-react";
import { latestAlbumsQueryOpts } from "@/lib/query-opts";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import AlbumCard from "@/components/AlbumCard";

export default function LatestAlbums() {
  const { data, isPending } = useQuery(latestAlbumsQueryOpts());

  const albums = data && !data.error ? data.data.albums : [];
  const hasError = data && data.error;

  return (
    <section
      role='region'
      aria-labelledby='recent-albums'
      aria-label='Recently Added Albums'
      className='mt-8'
    >
      <h2
        id='recent-albums'
        className='mb-4 text-xl font-semibold tracking-tight md:text-2xl'
      >
        Recently Added Albums
      </h2>

      {isPending ? (
        <div
          className='flex items-center justify-center py-12'
          role='status'
          aria-label='Loading albums...'
        >
          <Spinner className='size-8 text-amber-400' />
          <span className='sr-only'>Loading albums...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant='destructive'
          className='border-red-500/20 bg-red-500/10 text-red-400'
        >
          <AlertCircle className="size-4" aria-hidden="true" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>
            {data.message || "Failed to load albums. Please try again later."}
          </AlertDescription>
        </Alert>
      ) : albums.length > 0 ? (
        <>
          <span
            tabIndex={0}
            className='sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-slate-800 focus:px-4 focus:py-2 focus:text-white'
            aria-label={`Recently Added Albums section, ${albums.length} albums`}
          >
            Recently Added Albums - {albums.length} albums
          </span>
        <div className='grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6'>
          {albums.map(album => (
            <AlbumCard key={album.id} album={album} />
          ))}
        </div>
        </>
      ) : (
        <div className='py-12 text-center'>
          <div className='mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-slate-800'>
            <Music className="size-6 text-slate-600" aria-hidden="true" />
          </div>
          <h3 className='mb-2 text-lg font-semibold text-slate-300'>
            No Albums Yet
          </h3>
          <p className='mx-auto max-w-md text-slate-400'>
            Your music library is empty. Add some albums to get started with
            your personal music collection.
          </p>
        </div>
      )}
    </section>
  );
}
