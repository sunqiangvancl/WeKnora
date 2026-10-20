import type { RuntimeTask } from '@/api/system'

/**
 * The task drawer's detail rows, extracted from RuntimeQueues.vue so
 * `npx tsx --test` can pin them without mounting the SFC (same convention as
 * runtimeTaskPagination.ts).
 *
 * Every row is keyed by a translation key; the SFC resolves it with `t()`.
 * Rows that carry a resolved `name` render the name above the raw ID — the
 * owner's requirement that every ID field shows its human-readable name.
 */

export interface RuntimeTaskMetaRow {
  key: string
  labelKey: string
  value: string
  name?: string
}

export interface RuntimeTaskModelRow {
  key: string
  roleKey: string
  id: string
  name?: string
}

const TASKS_ROOT = 'system.globalSettings.runtime.tasks'

/** Kept in sync with internal/types/runtime_task_failure.go. */
export const RUNTIME_TASK_FAILURE_CODES = [
  'MODEL_RATE_LIMITED',
  'MODEL_UPSTREAM_ERROR',
  'MODEL_AUTH_FAILED',
  'MODEL_NOT_FOUND',
  'MODEL_EMPTY_OUTPUT',
  'MODEL_RESPONSE_INVALID',
  'DOCREADER_UNAVAILABLE',
  'DOCREADER_TIMEOUT',
  'DOCREADER_PARSE_FAILED',
  'SOURCE_FILE_UNREADABLE',
  'TASK_TIMEOUT',
  'TASK_CANCELED',
  'TASK_INTERRUPTED',
  'VECTORSTORE_WRITE_FAILED',
  'EMBEDDING_FAILED',
  'RESOURCE_GONE',
  'UNKNOWN',
] as const

/** Kept in sync with the RuntimeTaskModelRole* constants in the backend. */
export const RUNTIME_TASK_MODEL_ROLES = ['model', 'chat', 'embedding', 'summary', 'asr', 'vlm'] as const

export const RUNTIME_TASK_FAILURE_DISPOSITIONS = ['retryable', 'deterministic', 'unknown'] as const

function trimmed(value?: string): string | undefined {
  const result = value?.trim()
  return result ? result : undefined
}

export function runtimeTaskFailureReasonKey(code: string): string {
  const known = (RUNTIME_TASK_FAILURE_CODES as readonly string[]).includes(code)
  return `${TASKS_ROOT}.failureCodes.${known ? code : 'UNKNOWN'}`
}

export function runtimeTaskFailureDispositionKey(disposition?: string): string | null {
  if (!disposition) return null
  const known = (RUNTIME_TASK_FAILURE_DISPOSITIONS as readonly string[]).includes(disposition)
  return `${TASKS_ROOT}.failureDispositions.${known ? disposition : 'unknown'}`
}

export function runtimeTaskModelRoleKey(role: string): string {
  const known = (RUNTIME_TASK_MODEL_ROLES as readonly string[]).includes(role)
  return `${TASKS_ROOT}.modelRoles.${known ? role : 'model'}`
}

/** One row per model the task calls, with the resolved name and the raw ID. */
export function runtimeTaskModelRows(task: RuntimeTask): RuntimeTaskModelRow[] {
  return (task.models ?? [])
    .filter((model) => model.id)
    .map((model, index) => ({
      key: `model-${index}`,
      roleKey: runtimeTaskModelRoleKey(model.role),
      id: model.id,
      name: trimmed(model.name),
    }))
}

export function runtimeTaskMetaRows(task: RuntimeTask): RuntimeTaskMetaRow[] {
  const rows: RuntimeTaskMetaRow[] = []
  const kbClone = task.type === 'kb:clone'
  if (task.knowledge_base_id) {
    rows.push({
      key: 'kb',
      labelKey: `${TASKS_ROOT}.knowledgeBaseLabel`,
      value: task.knowledge_base_id,
      name: trimmed(task.knowledge_base_name),
    })
  }
  if (task.knowledge_id) {
    rows.push({
      key: 'knowledge',
      labelKey: `${TASKS_ROOT}.knowledgeLabel`,
      value: task.knowledge_id,
      // The upload name is the display fallback once the document row is
      // gone (the backend resolves the row first and fills this only from
      // the task payload).
      name: trimmed(task.knowledge_name) ?? trimmed(task.knowledge_file_name),
    })
  }
  if (task.task_id) {
    rows.push({ key: 'task', labelKey: `${TASKS_ROOT}.taskIDLabel`, value: task.task_id })
  }
  if (task.source_id) {
    rows.push({
      key: 'source',
      labelKey: `${TASKS_ROOT}.sourceLabel`,
      value: task.source_id,
      // kb:clone carries a knowledge base id in the generic source slot.
      name: kbClone ? trimmed(task.source_kb_name) : undefined,
    })
  }
  if (task.target_id) {
    rows.push({
      key: 'target',
      labelKey: `${TASKS_ROOT}.targetLabel`,
      value: task.target_id,
      name: kbClone ? trimmed(task.target_kb_name) : undefined,
    })
  }
  if (task.source_kb_id) {
    rows.push({
      key: 'source-kb',
      labelKey: `${TASKS_ROOT}.sourceKBLabel`,
      value: task.source_kb_id,
      name: trimmed(task.source_kb_name),
    })
  }
  if (task.target_kb_id) {
    rows.push({
      key: 'target-kb',
      labelKey: `${TASKS_ROOT}.targetKBLabel`,
      value: task.target_kb_id,
      name: trimmed(task.target_kb_name),
    })
  }
  if (task.data_source_id) {
    rows.push({
      key: 'datasource',
      labelKey: `${TASKS_ROOT}.dataSourceLabel`,
      value: task.data_source_id,
      name: trimmed(task.data_source_name),
    })
  }
  if (task.sync_log_id) {
    rows.push({ key: 'sync-log', labelKey: `${TASKS_ROOT}.syncLogLabel`, value: task.sync_log_id })
  }
  if (task.knowledge_count) {
    rows.push({ key: 'knowledge-count', labelKey: `${TASKS_ROOT}.knowledgeCountLabel`, value: String(task.knowledge_count) })
  }
  if (task.tenant_id) {
    rows.push({
      key: 'tenant',
      labelKey: `${TASKS_ROOT}.tenantLabel`,
      value: String(task.tenant_id),
      name: trimmed(task.tenant_name),
    })
  }
  return rows
}