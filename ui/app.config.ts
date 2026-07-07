// Nuxt UI runtime theme config.
//
// Nuxt UI 4 uses the `lucide` Iconify collection by default. We install it
// locally (@iconify-json/lucide) and bundle it (see `icon` config in
// nuxt.config.ts), so these defaults resolve from the bundle, not the CDN.
export default defineAppConfig({
  ui: {
    colors: {
      primary: 'blue',
      neutral: 'slate',
    },
  },
})
