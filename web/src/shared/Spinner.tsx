import type { Component } from "solid-js";

const Spinner: Component = () => (
  <div role='status' class='flex justify-center items-center'>
    <span class='sr-only'>loading</span>
    <div class='animate-spin rounded-full h-32 w-32 border-b-2 border-gray-900'></div>
  </div>
);

export default Spinner;
