import { Outlet } from "react-router-dom";
import AuthProvider from "../context/AuthContext";
import Header from "../shared/Header";

export default function RoutesLayout() {
  return (
    <AuthProvider>
      <Header />
      <main className='pt-5 mt-4'>
        <Outlet />
      </main>
    </AuthProvider>
  );
}
