import type { Component } from "solid-js";
import LatestMovies from "./LatestMovies";

const HomePage: Component = () => (
  <div class='min-h-screen'>
    <div class='container mx-auto px-4 py-8'>
      <div class='bg-info rounded-lg shadow-lg p-8 mb-8'>
        <h1 class='text-4xl md:text-6xl font-bold text-center text-light'>
          Igloo Media Center
        </h1>
      </div>

      <LatestMovies />
    </div>
  </div>
);

export default HomePage;
