import nativewind from "nativewind/preset";

/** @type {import('tailwindcss').Config} */
export default {
  content: ["./app/**/*.{js,ts,jsx,tsx}", "./components/**/*.{js,ts,jsx,tsx}"],
  presets: [nativewind],
  theme: {
    extend: {
      container: {
        center: true,
      },
      colors: {
        primary: "#1C395E",
        secondary: "#88BCF4",
        success: "#3E8AE1",
        danger: "#FDAC00",
        info: "#CEE3F9",
        light: "#F3F0E8",
        dark: "#121F32",
      },
    },
  },
  plugins: [],
};
