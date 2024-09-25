import type { Component } from "solid-js";
import LatestMovies from "./LatestMovies";

const HomePage: Component = () => (
  <div className='min-h-screen'>
    <div className='container mx-auto px-4 py-8'>
      <div className='bg-info rounded-lg shadow-lg p-8 mb-8'>
        <h1 className='text-4xl md:text-6xl font-bold text-center text-light'>
          Igloo Media Center
        </h1>
      </div>

      <LatestMovies />
    </div>
  </div>
);

export default HomePage;
