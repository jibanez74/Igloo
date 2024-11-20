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
      keyframes: {
        pulse: {
          '0%, 100%': { opacity: 1 },
          '50%': { opacity: .5 },
        },
      },
      animation: {
        pulse: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [],
};
