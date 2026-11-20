import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig(({ mode }) => {
  const configuredMockAuth = process.env.VITE_ENABLE_MOCK_AUTH;
  const enableMockAuth = configuredMockAuth === undefined
    ? mode === 'development'
    : configuredMockAuth === 'true';

  return {
    plugins: [react()],
    define: {
      __ENABLE_MOCK_AUTH__: JSON.stringify(enableMockAuth),
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: 'http://localhost:3000',
          changeOrigin: true,
        },
        '/auth': {
          target: 'http://localhost:8090',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/auth/, ''),
        },
      },
    },
  };
});
