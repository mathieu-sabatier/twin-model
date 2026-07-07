// Pinia draft store — read side + lifecycle. The single source of model truth
// for the UI. Wraps the Task-2 API client; never trusts a locally-mutated AST
// (determinism contract: after every future edit the server is re-fetched via
// loadModel() and the returned AST is used verbatim).
//
// Testability: the store factory accepts an optional `ApiClient` parameter.
// In the app, components call `useDraftStore()` with no argument and the store
// uses `useApi()` (the Nuxt composable). In tests, a fake client is injected
// directly so no MSW/Nuxt runtime is needed (MSW node interceptor does not
// fire under the Nuxt test env's $fetch — Task 2 report concern #3).
import { defineStore } from 'pinia'
import { ref, computed, toRaw } from 'vue'
import { useApi, type ApiClient, ProposeConflictError, type ProposeBody } from '~/api'
import type { Change, Diagnostic, Enum, Import, Instance, Model, ObjectType, Perspective, RepoInfo, ResolvedMember, UnitInfo } from '~/types'
import { indexDiagnostics, type DiagnosticIndex } from '~/lib/diagnosticPath'
import { emitModel } from '~/lib/emitYaml'

// ── Debounce helper ──────────────────────────────────────────────────────────
// Minimal manual debounce so this module stays free of composable context
// (vueuse/useDebounceFn requires Vue reactive context; the store setup function
// runs inside Vue/Pinia but a plain closure is simpler and 100% testable with
// vi.useFakeTimers without any special vueuse wiring).
function debounce(fn: () => void, ms: number): () => void {
  let timer: ReturnType<typeof setTimeout> | null = null
  return () => {
    if (timer !== null) clearTimeout(timer)
    timer = setTimeout(() => {
      timer = null
      fn()
    }, ms)
  }
}

// ── Store factory ────────────────────────────────────────────────────────────

/**
 * useDraftStore — the app's read-side draft store.
 *
 * Call with no arguments in components (uses useApi() from Nuxt runtime).
 * Pass a fake ApiClient in tests to bypass the Nuxt $fetch layer.
 *
 * Pinia caches store definitions by the string id 'draft': a second call to
 * defineStore('draft', ...) within the same Pinia instance returns the cached
 * version and ignores the new setup function / injected client. Tests rely on
 * createPinia() in beforeEach to get a fresh Pinia instance per test, which
 * re-runs the setup function and picks up the new fake client. In the app only
 * useDraftStore() (no argument) is ever called, so there is always exactly one
 * client: the one returned by useApi().
 */
export const useDraftStore = (injectedClient?: ApiClient) =>
  _defineAndUse(injectedClient)

// The inner define+use is wrapped so we can call defineStore lazily with the
// injected client captured in closure. Each unique client reference gets its own
// store id (tests always pass a fresh client per describe block).
function _defineAndUse(injectedClient?: ApiClient) {
  const store = defineStore('draft', () => {
    // Resolve the client once at store-setup time.
    // In the app: useApi() (Nuxt runtime). In tests: the injected fake.
    const api: ApiClient = injectedClient ?? useApi()

    // ── State ────────────────────────────────────────────────────────────────

    const draftId = ref<string | null>(null)
    const baseRef = ref<string>('main')
    const file = ref<string | null>(null)
    const files = ref<string[]>([])
    const model = ref<Model | null>(null)
    const diagnostics = ref<Diagnostic[]>([])
    const parseError = ref<string | null>(null)
    /** True after a successful propose — the draft is frozen, edits not allowed. */
    const frozen = ref(false)
    const loading = ref(false)
    const error = ref<string | null>(null)
    const yaml = ref<string | null>(null)
    const diagramSrc = ref<string | null>(null)
    const units = ref<UnitInfo[]>([])
    // Cache of resolved (inherited-flattened) members per type name, for instance forms.
    const resolvedCache = ref<Record<string, ResolvedMember[]>>({})

    // ── Repo context / branch picker state (Task 4 — footer) ────────────────
    const repo = ref<RepoInfo | null>(null)
    const branchOptions = ref<string[]>([])

    // ── Diff / propose state ─────────────────────────────────────────────────
    const changes = ref<Change[]>([])
    const diffText = ref<string>('')

    /** True while a saveModel round-trip is in flight. Used by UI to disable editable controls. */
    const saving = ref(false)

    // ── Computed getters ─────────────────────────────────────────────────────

    /** Memoised diagnostic index recomputed whenever diagnostics change. O(1) lookups. */
    const diagnosticIndex = computed<DiagnosticIndex>(() =>
      indexDiagnostics(diagnostics.value),
    )

    const errorCount = computed<number>(
      () => diagnostics.value.filter((d) => d.severity === 'error').length,
    )

    /** True when there are error-severity diagnostics — drives Propose button gate. */
    const hasErrors = computed<boolean>(() => errorCount.value > 0)

    /** Convenience accessors over model — avoids optional-chaining at every call site. */
    const objectTypes = computed<ObjectType[]>(() => model.value?.objectTypes ?? [])
    const instances = computed<Instance[]>(() => model.value?.instances ?? [])
    const enums = computed<Enum[]>(() => model.value?.enums ?? [])
    const imports = computed<Import[]>(() => model.value?.imports ?? [])
    const perspectives = computed<Perspective[]>(() => model.value?.perspectives ?? [])

    // ── Internal helpers ─────────────────────────────────────────────────────

    /** Re-fetch model from server (the canonical re-render source). */
    async function loadModel(): Promise<void> {
      if (!draftId.value || !file.value) return
      loading.value = true
      error.value = null
      try {
        const res = await api.getDraftModel(draftId.value, file.value)
        model.value = res.model
        diagnostics.value = res.diagnostics ?? []
        parseError.value = res.parseError ?? null
        resolvedCache.value = {}
      } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : String(err)
      } finally {
        loading.value = false
      }
    }

    // ── Actions ──────────────────────────────────────────────────────────────

    /**
     * Create a new draft from baseRef and load its model. Returns the draft id
     * so the caller can put it in the route (no localStorage — determinism contract).
     */
    async function createDraft(base = 'main'): Promise<string> {
      baseRef.value = base
      error.value = null
      try {
        const res = await api.createDraft(base)
        draftId.value = res.id
        files.value = res.files
        file.value = res.files[0] ?? null
        await loadModel()
        return res.id
      } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : String(err)
        throw err
      }
    }

    /**
     * True only for a 404 whose body says the *draft* is gone — not a file-level
     * 404. Recreating a draft on a file-not-found 404 caused the H1 loop, so we
     * gate recreation strictly on the draft-missing signal.
     */
    function isDraftNotFound(err: unknown): boolean {
      const e = err as
        | { status?: number; response?: { status?: number }; data?: { error?: string } }
        | null
      const is404 = !!e && (e.status === 404 || e.response?.status === 404)
      if (!is404) return false
      const msg = e?.data?.error ?? ''
      // Default to "draft missing" only when the message clearly is not file-scoped,
      // so a plain 404 with no body still recovers gracefully.
      return !msg.includes('file not found')
    }

    /**
     * Recover a draft by id after a refresh / deep link. If the stored id no
     * longer exists on the server (404 — e.g. the server restarted and dropped
     * in-memory drafts), transparently create a fresh draft instead of leaving
     * the user on a broken page with a flood of 404s. Returns the id actually in
     * use so the caller can pin it into the URL. QA finding H3.
     */
    async function recoverOrCreate(id: string): Promise<string> {
      draftId.value = id
      loading.value = true
      error.value = null
      try {
        const res = await api.getDraftModel(id)
        // Restore the file switcher on refresh / deep-link: unlike createDraft,
        // this recovery path has no CreateDraftResponse, so the file list rides
        // along on the model response (empty only for an unexpectedly fileless draft).
        if (res.files?.length) files.value = res.files
        file.value = res.file
        model.value = res.model
        diagnostics.value = res.diagnostics ?? []
        parseError.value = res.parseError ?? null
        resolvedCache.value = {}
        return id
      } catch (err: unknown) {
        if (isDraftNotFound(err)) {
          // Stale draft id — mint a new draft. createDraft sets draftId/files/file/model.
          return await createDraft('main')
        }
        error.value = err instanceof Error ? err.message : String(err)
        throw err
      } finally {
        loading.value = false
      }
    }

    /**
     * Recover an existing draft after a page refresh. The id comes from the route
     * (no localStorage). Re-fetches the model from the server.
     */
    async function loadDraft(id: string, f?: string): Promise<void> {
      draftId.value = id
      if (f !== undefined) file.value = f
      await loadModel()
    }

    /** Switch the active model file and refresh every view derived from it. */
    async function setFile(f: string): Promise<void> {
      if (f === file.value) return
      file.value = f
      yaml.value = null
      diagramSrc.value = null
      await loadModel()
    }

    /**
     * Run validation immediately and update diagnostics/parseError.
     * The editor will call scheduleValidate() after edits; this is the raw action.
     */
    async function validateNow(): Promise<void> {
      if (!draftId.value) return
      try {
        const res = await api.validate(draftId.value, file.value ?? undefined)
        diagnostics.value = res.diagnostics ?? []
        parseError.value = res.parseError ?? null
      } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : String(err)
      }
    }

    // L3: skip a preview fetch when nothing that affects it changed. The model
    // ref changes on every server (re)load, so a post-edit fetch still refreshes.
    let lastYaml: { draft: string; file: string; model: Model | null } | null = null
    let lastDiagram: { draft: string; file: string; view: string; model: Model | null } | null = null

    /** Fetch the selected file's canonical YAML text for the YAML pane. */
    async function loadYaml(): Promise<void> {
      if (!draftId.value || !file.value) return
      if (lastYaml && lastYaml.draft === draftId.value && lastYaml.file === file.value && lastYaml.model === model.value) return
      lastYaml = { draft: draftId.value, file: file.value, model: model.value }
      try { yaml.value = await api.getDraftFile(draftId.value, file.value) }
      // Clear the key on failure so a transient error doesn't lock out a retry.
      catch (err: unknown) { console.warn('YAML preview failed:', err); yaml.value = null; lastYaml = null }
    }

    /** Fetch the Mermaid diagram source (types by default; instances topology on demand). */
    async function loadDiagram(view: 'types' | 'instances' = 'types'): Promise<void> {
      if (!draftId.value || !file.value) return
      if (lastDiagram && lastDiagram.draft === draftId.value && lastDiagram.file === file.value && lastDiagram.view === view && lastDiagram.model === model.value) return
      lastDiagram = { draft: draftId.value, file: file.value, view, model: model.value }
      try { diagramSrc.value = await api.previewDiagram(draftId.value, file.value, view) }
      // Clear the key on failure so a transient error doesn't lock out a retry.
      catch (err: unknown) { console.warn('diagram preview failed:', err); diagramSrc.value = null; lastDiagram = null }
    }

    /**
     * Persist an edited model: emit parseable YAML, PUT it (the server
     * canonicalizes), then re-fetch the canonical AST + diagnostics. The local
     * `next` is never trusted as the render source (determinism contract).
     *
     * Sets `saving` for the duration of the round-trip so the UI can disable
     * editable controls and prevent overlapping mutations. On failure, sets
     * `error` and re-throws so dialog/slideover callers can still catch.
     */
    async function saveModel(next: Model): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot save')
      if (!draftId.value || !file.value) throw new Error('no active draft')
      error.value = null
      saving.value = true
      try {
        const yamlText = emitModel(next)
        await api.putFiles(draftId.value, { [file.value]: yamlText })
        await loadModel()
      } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : String(err)
        throw err
      } finally {
        saving.value = false
      }
    }

    /**
     * Load available engineering units from the API. Units are static config;
     * subsequent calls are no-ops (idempotent).
     */
    async function loadUnits(): Promise<void> {
      if (units.value.length) return
      try { units.value = await api.getUnits() }
      catch (err: unknown) { error.value = err instanceof Error ? err.message : String(err) }
    }

    async function resolvedFor(type: string): Promise<ResolvedMember[]> {
      if (resolvedCache.value[type]) return resolvedCache.value[type]
      if (!draftId.value) return []
      const res = await api.resolved(draftId.value, type, file.value ?? undefined)
      resolvedCache.value = { ...resolvedCache.value, [type]: res.members }
      return res.members
    }

    /** Append a new instance and persist. Server validates name/parent on reload. */
    async function createInstance(input: { name: string; type: string; under: string }): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot create')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      const zero = { file: '', line: 0, col: 0 }
      ;(next.instances ??= []).push({
        pos: zero,
        name: input.name,
        type: { raw: input.type, name: input.type, pos: zero },
        under: { raw: input.under, name: input.under, pos: zero },
      })
      await saveModel(next)
    }

    /**
     * Ensure `next.imports` contains `uri`. Idempotent: if the URI is already
     * imported (under any alias) nothing changes; otherwise a new
     * `{alias, uri}` entry is appended. Mutates `next` in place (the caller
     * passes a clone bound for saveModel).
     */
    function ensureImport(next: Model, alias: string, uri: string): void {
      const imports = (next.imports ??= [])
      if (imports.some((i) => i.uri === uri)) return
      imports.push({ pos: { file: '', line: 0, col: 0 }, alias, uri })
    }

    /** Resolve the alias a given URI is imported under, or the supplied default. */
    function aliasForUri(next: Model, uri: string, fallback: string): string {
      const hit = (next.imports ?? []).find((i) => i.uri === uri)
      return hit ? hit.alias : fallback
    }

    /** Create a local ObjectType based on a companion type; adds the import. */
    async function extendType(input: { name: string; baseAlias: string; baseName: string; baseUri: string }): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot create')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      ensureImport(next, input.baseAlias, input.baseUri)
      const alias = aliasForUri(next, input.baseUri, input.baseAlias)
      const raw = `${alias}:${input.baseName}`
      const zero = { file: '', line: 0, col: 0 }
      ;(next.objectTypes ??= []).push({
        pos: zero,
        name: input.name,
        base: { pos: zero, alias, name: input.baseName, raw },
        members: [],
      })
      await saveModel(next)
    }

    /** Add an instance of a companion (or companion-derived) type; adds the import. */
    async function addCompanionInstance(input: { name: string; under: string; typeAlias: string; typeName: string; typeUri: string }): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot create')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      ensureImport(next, input.typeAlias, input.typeUri)
      const alias = aliasForUri(next, input.typeUri, input.typeAlias)
      const raw = `${alias}:${input.typeName}`
      const zero = { file: '', line: 0, col: 0 }
      ;(next.instances ??= []).push({
        pos: zero,
        name: input.name,
        type: { pos: zero, alias, name: input.typeName, raw },
        under: { pos: zero, name: input.under, raw: input.under },
      })
      await saveModel(next)
    }

    /** Set a local type member's `type:` to a companion type ref; adds the import. */
    async function setMemberType(input: { typeName: string; member: string; refAlias: string; refName: string; refUri: string }): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot edit')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      ensureImport(next, input.refAlias, input.refUri)
      const alias = aliasForUri(next, input.refUri, input.refAlias)
      const ot = (next.objectTypes ?? []).find((t) => t.name === input.typeName)
      if (!ot) throw new Error(`type not found: ${input.typeName}`)
      const mem = (ot.members ?? []).find((m) => m.name === input.member)
      if (!mem) throw new Error(`member not found: ${input.member}`)
      const zero = { file: '', line: 0, col: 0 }
      mem.type = { pos: zero, alias, name: input.refName, raw: `${alias}:${input.refName}` }
      await saveModel(next)
    }

    /** True if an instance with this name already exists (trimmed compare). */
    function nameTaken(name: string): boolean {
      const n = name.trim()
      return (model.value?.instances ?? []).some((i) => i.name === n)
    }

    /** Remove an instance by name and persist. */
    async function deleteInstance(name: string): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot delete')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      next.instances = (next.instances ?? []).filter((i) => i.name !== name.trim())
      await saveModel(next)
    }

    /** Rename an instance and persist. */
    async function renameInstance(oldName: string, newName: string): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot rename')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      const inst = (next.instances ?? []).find((i) => i.name === oldName.trim())
      if (!inst) throw new Error(`instance not found: ${oldName}`)
      // inst is inside next (the clone), not model.value — determinism contract holds
      inst.name = newName.trim()
      await saveModel(next)
    }

    /** Re-parent an instance: set its `under` ref and persist (one-line diff). */
    async function reparentInstance(name: string, underRaw: string): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot re-parent')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      const inst = (next.instances ?? []).find((i) => i.name === name)
      if (!inst) throw new Error(`instance not found: ${name}`)
      const alias = underRaw.includes(':') ? underRaw.split(':')[0] : undefined
      const nm = underRaw.includes(':') ? underRaw.split(':')[1]! : underRaw
      inst.under = { raw: underRaw, name: nm, pos: { file: '', line: 0, col: 0 }, ...(alias ? { alias } : {}) }
      await saveModel(next)
    }

    /** Add or remove an instance from a perspective node's `members` and persist. */
    async function assignMembership(input: { perspective: string; node: string; instance: string; mode: 'add' | 'remove' }): Promise<void> {
      if (frozen.value) throw new Error('draft is frozen; cannot edit membership')
      const next = structuredClone(toRaw(model.value))
      if (!next) throw new Error('no model loaded')
      const p = (next.perspectives ?? []).find((x) => x.id === input.perspective)
      const nd = p?.nodes?.find((x) => x.id === input.node)
      if (!nd) throw new Error(`perspective node not found: ${input.perspective}/${input.node}`)
      const members = new Set(nd.members ?? [])
      if (input.mode === 'add') members.add(input.instance)
      else members.delete(input.instance)
      nd.members = [...members]
      await saveModel(next)
    }

    /** Fetch the semantic diff for the current draft and populate changes/diffText. */
    async function loadDiff(): Promise<void> {
      if (!draftId.value) return
      const res = await api.diff(draftId.value, file.value ?? undefined)
      // Server returns `changes: null` when there is no *semantic* change (doc/
      // comment/format-only edits). Coerce to [] so the slideover renders its
      // "No changes." empty state instead of crashing on `.length`. QA finding H1.
      changes.value = res.changes ?? []
      diffText.value = res.text ?? ''
    }

    /** Load repo context (owner/repo, commit identity, propose availability).
     * Idempotent; degrades silently — a failure just leaves the footer chip hidden. */
    async function loadRepo(): Promise<void> {
      if (repo.value) return
      try {
        repo.value = await api.getRepo()
      } catch (err: unknown) {
        console.warn('repo info failed:', err)
      }
    }

    /** Build the branch picker options from the repo's real branches (GET
     * /api/branches): default branch first, plus the current base, deduped and
     * order-preserving. Degrades silently to default + base on failure — the
     * picker is never empty and never raises a user-facing error. */
    async function loadBranchOptions(): Promise<void> {
      const opts = new Set<string>()
      try {
        const list = await api.listBranches()
        opts.add(list.defaultBranch || 'main')
        if (baseRef.value) opts.add(baseRef.value)
        for (const b of list.branches) {
          if (b) opts.add(b)
        }
      } catch (err: unknown) {
        console.warn('branch list failed:', err)
        opts.clear()
        opts.add(repo.value?.defaultBranch ?? 'main')
        if (baseRef.value) opts.add(baseRef.value)
      }
      branchOptions.value = [...opts]
    }

    /** Switch the draft's base branch: create a fresh draft off `branch` and
     * return its id (the caller repoints the URL). No confirmation. On failure
     * (e.g. the branch does not exist) keep the current draft, restore the prior
     * branch, and set a humane error; return null. */
    async function switchBranch(branch: string): Promise<string | null> {
      const target = branch.trim()
      if (!target || target === baseRef.value) return null
      const prevBaseRef = baseRef.value
      try {
        return await createDraft(target)
      } catch (err: unknown) {
        baseRef.value = prevBaseRef
        const data = (err as { data?: { error?: string } } | null)?.data
        error.value = data?.error ?? `Could not switch to branch "${target}"`
        return null
      }
    }

    /**
     * Propose the draft as a pull request. On success freezes the draft and
     * returns the PR URL. On 409 (lint-red) populates diagnostics from the
     * conflict error and rethrows.
     */
    async function propose(body: ProposeBody): Promise<string> {
      if (!draftId.value) throw new Error('no active draft')
      try {
        const res = await api.propose(draftId.value, body)
        frozen.value = true
        return res.url
      } catch (err) {
        if (err instanceof ProposeConflictError) diagnostics.value = err.diagnostics
        throw err
      }
    }

    // ── Debounced validate ───────────────────────────────────────────────────
    // 300ms debounce — the editor calls this after every keystroke. Multiple rapid
    // calls collapse into one network request.
    const scheduleValidate = debounce(() => void validateNow(), 300)

    return {
      // State (expose refs so they are reactive in components)
      draftId,
      baseRef,
      file,
      files,
      model,
      diagnostics,
      parseError,
      frozen,
      loading,
      saving,
      error,
      yaml,
      diagramSrc,
      units,
      changes,
      diffText,
      repo,
      branchOptions,
      // Getters
      diagnosticIndex,
      errorCount,
      hasErrors,
      objectTypes,
      instances,
      enums,
      imports,
      perspectives,
      // Actions
      createDraft,
      loadDraft,
      recoverOrCreate,
      setFile,
      loadModel,
      validateNow,
      scheduleValidate,
      loadYaml,
      loadDiagram,
      saveModel,
      loadUnits,
      resolvedFor,
      createInstance,
      ensureImport,
      extendType,
      addCompanionInstance,
      setMemberType,
      nameTaken,
      deleteInstance,
      renameInstance,
      reparentInstance,
      assignMembership,
      loadDiff,
      propose,
      loadRepo,
      loadBranchOptions,
      switchBranch,
    }
  })

  return store()
}
