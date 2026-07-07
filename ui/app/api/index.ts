// Public entrypoint for the one-and-only HTTP client. Import from `~/api`.
export {
  createApiClient,
  ProposeConflictError,
  ProposeError,
  type ApiClient,
  type CreateDraftResponse,
  type JsonSchema,
  type ProposeBody,
  type ProposeResponse,
} from './client'
export { useApi } from './useApi'
