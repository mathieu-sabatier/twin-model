<script setup lang="ts">
// App root. Renders the routed page inside the Nuxt UI provider. The draft id
// lives in the route (pages/[[draftId]].vue) — there is no localStorage; a refresh
// recovers the draft by re-fetching from the server. Product chrome lives in
// components/AppShell.vue, mounted by the page.
useHead({
  title: 'twinmodel',
  meta: [{ name: 'description', content: 'Digital twin modeler' }],
  link: [{ rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
})
</script>

<template>
  <UApp>
    <NuxtPage />
  </UApp>
</template>
