{
  /* Admin Toggle */
}
<div className='flex items-center justify-between py-2'>
  <div className='flex items-center gap-2'>
    <FiShield className='h-5 w-5 text-sky-400/50' aria-hidden='true' />
    <div>
      <label htmlFor='isAdmin' className='text-sm font-medium text-sky-200'>
        Admin User
      </label>
      <p className='text-xs text-sky-200/70' id='admin-description'>
        Grant administrative privileges to this user
      </p>
    </div>
  </div>
  <div className='relative'>
    <input
      type='checkbox'
      id='isAdmin'
      name='isAdmin'
      disabled={isLoading}
      className='sr-only peer'
      aria-describedby='admin-description'
      role='switch'
      aria-checked='false'
    />
    <label
      htmlFor='isAdmin'
      className={`relative inline-flex h-6 w-11 items-center rounded-full 
                           bg-slate-900/50 cursor-pointer transition-colors
                           peer-focus:outline-none peer-focus:ring-2 
                           peer-focus:ring-sky-500/40 peer-focus:ring-offset-2
                           peer-focus:ring-offset-slate-800
                           peer-checked:bg-sky-500/50
                           ${isLoading ? "cursor-not-allowed opacity-50" : ""}`}
    >
      <span className='sr-only'>Enable admin privileges</span>
      <span
        aria-hidden='true'
        className={`inline-block h-5 w-5 transform rounded-full 
                             bg-white transition-transform
                             peer-checked:translate-x-5`}
      />
    </label>
  </div>
</div>;
