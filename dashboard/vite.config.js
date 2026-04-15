import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/dashboard/',
  server: {
    port: 3001,
    proxy: {
      '/dashboard/events': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/dashboard/graph': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/dashboard/stats': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/dashboard/chat': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
