import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { enhancedImages } from '@sveltejs/enhanced-img';

export default defineConfig({
  plugins: [sveltekit(), enhancedImages()],

  server: {
    port: 3000,
    open: true,
    proxy: {
      '/api/v1': {
        target: 'http://localhost:8000/api/v1',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/v1/, '') // Remove '/api/v1' from the request path
      }
    }
  }
});
