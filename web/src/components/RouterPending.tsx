export default function RouterPending() {
  return (
    <div
      className='flex min-h-[50vh] items-center justify-center'
      role='status'
      aria-label='Loading page'
    >
      <div className='flex flex-col items-center gap-4'>
        {/* Animated spinner */}
        <div className='relative'>
          <div className='h-12 w-12 rounded-full border-4 border-slate-700' />
          <div className='absolute inset-0 h-12 w-12 animate-spin rounded-full border-4 border-transparent border-t-amber-400' />
        </div>

        {/* Loading text */}
        <p className='text-sm font-medium text-slate-400'>Loading...</p>

        {/* Screen reader only text */}
        <span className='sr-only'>Please wait while the page loads</span>
      </div>
    </div>
  );
}

