import "./assets/styles.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import Login from "./components/Login";
import AuthProvider from "./context/AuthProvider";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <AuthProvider>
      <Login />
    </AuthProvider>
  </StrictMode>
);
