import { afterAll, afterEach, beforeAll } from 'vitest'
import { setupServer } from 'msw/node'
import { handlers, resetStore } from '~/mocks/handlers'

// MSW Node server for tests, seeded with the SAME handlers the dev browser worker
// uses (single source of truth for the mocked Go API). `onUnhandledRequest:'error'`
// makes any request a test forgot to mock a hard failure, keeping mocks honest.
export const server = setupServer(...handlers)

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => {
  server.resetHandlers()
  // The handlers share an in-memory draft store; reset it so tests are isolated.
  resetStore()
})
afterAll(() => server.close())
