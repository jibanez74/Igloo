/* @refresh reload */
import { render } from "solid-js/web";
import "./assets/css/styles.css";
import routes from "./routes";
import { Router } from "@solidjs/router";

const root = document.getElementById("root");

render(() => <Router>{routes}</Router>, root!);
