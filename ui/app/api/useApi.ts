// Nuxt composable wrapper around createApiClient. This is the ONLY place in the
// app that reads the runtime API base (useRuntimeConfig().public.apiBase). Kept
// separate from client.ts so the factory stays pure and unit-testable without a
// Nuxt runtime.
import { createApiClient, type ApiClient } from './client'

/**
 * The typed API client bound to the runtime base URL (NUXT_PUBLIC_API_BASE,
 * default `/api`). Use this everywhere in components/stores instead of `$fetch`.
 */
export function useApi(): ApiClient {
  // `useRuntimeConfig` is a Nuxt auto-import available at runtime.
  const config = useRuntimeConfig()
  return createApiClient(config.public.apiBase)
}
