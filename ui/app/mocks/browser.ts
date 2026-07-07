// MSW browser worker for DEV ONLY. Started by app/plugins/msw.client.ts and only
// when import.meta.dev is true and we are in the browser. It is NOT imported by
// any production code path, so tree-shaking keeps it (and msw/browser) out of the
// SSG production bundle. The real Go API serves /api in prod.
import { setupWorker } from 'msw/browser'
import { handlers } from './handlers'

/** The dev-only MSW worker seeded with the shared handlers. */
export const worker = setupWorker(...handlers)
