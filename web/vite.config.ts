import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy: { '/api': 'http://localhost:8095' } },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // three.js alone is ~700 kB minified; it lives in its own lazy chunk (3D view only), so allow that one.
    chunkSizeWarningLimit: 800,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            { name: 'three', test: /node_modules[\\/]three[\\/]/, priority: 30 },
            { name: 'r3f', test: /node_modules[\\/](@react-three|react-reconciler|three-stdlib|three-mesh-bvh|meshline|camera-controls|maath|stats-gl|troika[^\\/]*|detect-gpu|hls\.js|bidi-js|webgl-sdf-generator|potpack|@monogrid)[\\/]/, priority: 20 },
            { name: 'react', test: /node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/, priority: 40 },
            { name: 'motion', test: /node_modules[\\/](motion|motion-dom|motion-utils|framer-motion)[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
})
