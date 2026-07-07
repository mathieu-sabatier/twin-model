import { defineConfig } from 'vitest/config'
import { defineVitestProject } from '@nuxt/test-utils/config'

// A single "nuxt" project: tests run inside a real Nuxt runtime (happy-dom DOM)
// so @nuxt/test-utils + Nuxt UI auto-imports/components work. The MSW Node
// setupServer is started/stopped around every test via test/setup.ts.
export default defineConfig({
  test: {
    projects: [
      await defineVitestProject({
        test: {
          name: 'nuxt',
          environment: 'nuxt',
          include: ['test/**/*.{test,spec}.ts'],
          setupFiles: ['./test/setup.ts'],
        },
      }),
    ],
  },
})
