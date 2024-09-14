import { BrowserRouter, Routes, Route } from "react-router-dom";
import Navbar from "./shared/Navbar";
import HomePage from "./home/HomePage";
import MovieDetailsPage from "./movies/MovieDetailsPage";

export default function App() {
  return (
    <BrowserRouter>
      <header>
        <Navbar />
      </header>

      <main>
        <Routes>
          <Route element={<HomePage />} path='/' />

          <Route path='/movies/details/:id' element={<MovieDetailsPage />} />
        </Routes>
      </main>
    </BrowserRouter>
  );
}
