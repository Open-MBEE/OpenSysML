import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.dirname(fileURLToPath(import.meta.url));
const realSysON = process.env.OPENSYSML_SYSON_REAL === '1';
const peerDependencies = [
  '@apollo/client',
  '@eclipse-sirius/sirius-components-core',
  '@eclipse-sirius/sirius-components-trees',
  '@emotion/react',
  '@emotion/styled',
  '@mui/icons-material',
  '@mui/material',
  'graphql',
  'react',
  'react-dom',
];
const isExternal = (id: string) =>
  peerDependencies.some((dependency) => id === dependency || id.startsWith(`${dependency}/`));

export default defineConfig({
  plugins: [react()],
  resolve: realSysON
    ? undefined
    : {
        alias: {
          '@eclipse-sirius/sirius-components-core': path.resolve(root, 'src/vendor/sirius-components-core.tsx'),
          '@eclipse-sirius/sirius-components-trees': path.resolve(root, 'src/vendor/sirius-components-trees.ts'),
        },
      },
  build: {
    minify: false,
    lib: {
      name: 'opensysml-syson',
      entry: path.resolve(root, 'src/index.ts'),
      formats: ['es', 'cjs'],
      fileName: (format) => `opensysml-syson.${format}.js`,
    },
    rollupOptions: {
      external: (id) => isExternal(id) || (!realSysON && /^@eclipse-sirius\/sirius-components-(core|trees)(\/|$)/.test(id)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
});
