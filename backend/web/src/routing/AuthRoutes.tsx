import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

export default function AuthRoutes() {
  const { user } = useAuth();

  if (!user) {
    return <Navigate to='/login' replace />;
  }

  return <Outlet />;
}
