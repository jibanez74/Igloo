import type { Component } from "solid-js";

const Spinner: Component = () =>
  a(
    <div role='status' className='flex justify-center items-center'>
      <span className='sr-only'>loading</span>
      <div className='animate-spin rounded-full h-32 w-32 border-b-2 border-gray-900'></div>
    </div>
  );

export default Spinner;
