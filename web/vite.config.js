import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    open: true,
    port: 3000,
    proxy: {
      '/api/v1': {
        target: 'http://localhost:8080/api/v1',
        changeOrigin: true
      }
    }
  },
  plugins: [sveltekit()]
});
