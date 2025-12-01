import { useQuery } from "@tanstack/react-query";
import { getLatestAlbums } from "@/lib/api";
import { LATEST_ALBUMS_KEY } from "@/lib/constants";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import AlbumCard from "@/components/AlbumCard";

export default function LatestAlbums() {
  const { data, isPending } = useQuery({
    queryKey: [LATEST_ALBUMS_KEY],
    queryFn: getLatestAlbums,
  });

  const albums = data && !data.error ? data.data.albums : [];
  const hasError = data && data.error;

  return (
    <section aria-labelledby='recent-albums' className='mt-8'>
      <h2
        id='recent-albums'
        className='text-xl md:text-2xl font-semibold tracking-tight mb-4'
      >
        Recently Added Albums
      </h2>

      {isPending ? (
        <div
          className='py-12 flex items-center justify-center'
          role='status'
          aria-label='Loading albums...'
        >
          <Spinner className='size-8 text-amber-400' />
          <span className='sr-only'>Loading albums...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant='destructive'
          className='bg-red-500/10 border-red-500/20 text-red-400'
        >
          <i className='fa-solid fa-circle-exclamation' aria-hidden='true'></i>
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>
            {data.message || "Failed to load albums. Please try again later."}
          </AlertDescription>
        </Alert>
      ) : albums.length > 0 ? (
        <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
          {albums.map(album => (
            <AlbumCard key={album.id} album={album} />
          ))}
        </div>
      ) : (
        <div className='text-center py-12'>
          <div className='mx-auto w-16 h-16 bg-slate-800 rounded-full flex items-center justify-center mb-4'>
            <i className='fa-solid fa-music text-slate-600 text-2xl'></i>
          </div>
          <h3 className='text-lg font-semibold text-slate-300 mb-2'>
            No Albums Yet
          </h3>
          <p className='text-slate-400 max-w-md mx-auto'>
            Your music library is empty. Add some albums to get started with
            your personal music collection.
          </p>
        </div>
      )}
    </section>
  );
}
