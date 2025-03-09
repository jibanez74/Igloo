import { createContext } from "react";
import type { AuthContextType } from "../types/Auth";

const AuthContext = createContext<AuthContextType | null>(null);

export default AuthContext;
