import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'
import { vanillaExtractPlugin } from '@vanilla-extract/vite-plugin'

export default defineConfig({
  plugins: [preact(), vanillaExtractPlugin()],
  build: {
    outDir: '../api/ui/dist',
    emptyOutDir: true,
  },
})
