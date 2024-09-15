import { Outlet } from "react-router-dom";
import Navbar from "../shared/Navbar";

export default function RouterLayout() {
  return (
    <>
      <header>
        <Navbar />
      </header>

      <main>
        <Outlet />
      </main>
    </>
  );
}
