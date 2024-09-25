import { lazy } from "solid-js";

const routes = [
  {
    path: "/",
    component: lazy(() => import("./home/HomePage.tsx")),
  },
];

export default routes;
