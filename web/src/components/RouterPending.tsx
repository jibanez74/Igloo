export default function RouterPending() {
  return (
    <div
      className='flex items-center justify-center min-h-[50vh]'
      role='status'
      aria-label='Loading page'
    >
      <div className='flex flex-col items-center gap-4'>
        {/* Animated spinner */}
        <div className='relative'>
          <div className='w-12 h-12 rounded-full border-4 border-slate-700' />
          <div className='absolute inset-0 w-12 h-12 rounded-full border-4 border-transparent border-t-amber-400 animate-spin' />
        </div>

        {/* Loading text */}
        <p className='text-slate-400 text-sm font-medium'>Loading...</p>

        {/* Screen reader only text */}
        <span className='sr-only'>Please wait while the page loads</span>
      </div>
    </div>
  );
}

