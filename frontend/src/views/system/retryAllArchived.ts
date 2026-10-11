/**
 * Pure helpers behind the "重试全部失败 / Retry all failed" confirm dialog in
 * RuntimeQueues.vue. Extracted from the SFC so `npx tsx --test` can pin the
 * behaviour (the deploy box has no node; CI is the only verifier).
 */

export interface RetryAllFacetEntry {
  type: string
  count: number
}

export interface RetryAllResultSummary {
  retried: number
  failed: number
  skipped: number
  byType: Record<string, number>
}

/**
 * Deliberate product decision: failures of this task type are deterministic
 * in this deployment (the Excel reader cannot open certain streaming-zip
 * workbooks), so a retry burns parse + model calls and re-archives the task
 * immediately. The dialog keeps it UNCHECKED by default; every other type in
 * these queues is dominated by transient failures (provider 429 storms,
 * timeouts, an unavailable docreader) where retry genuinely works.
 */
export const DETERMINISTIC_RETRY_ALL_TYPE = 'datatable:summary'

/**
 * Normalize the facets endpoint's `by_type` map into dialog rows, biggest
 * first, with a code-point tiebreak so the checkbox order is stable across
 * reloads. The backend guarantees `{}` (never null) today; still tolerate
 * null/undefined rather than trusting the wire at the UI boundary.
 */
export function normalizeRetryAllFacets(
  byType: Record<string, number> | null | undefined,
): RetryAllFacetEntry[] {
  return Object.entries(byType ?? {})
    .map(([type, count]) => ({ type, count: Number.isFinite(Number(count)) ? Number(count) : 0 }))
    .sort((a, b) => b.count - a.count || (a.type < b.type ? -1 : a.type > b.type ? 1 : 0))
}

/** Default checked-set for the dialog: every facet type except the deterministic one. */
export function defaultRetryAllSelection(facets: RetryAllFacetEntry[]): string[] {
  return facets.filter((entry) => entry.type !== DETERMINISTIC_RETRY_ALL_TYPE).map((entry) => entry.type)
}

/**
 * Normalize the retry_all response. Its `by_type` is NOT guaranteed like the
 * facets endpoint's — it may arrive as null. `failed` counts ATTEMPTS that
 * errored, not distinct tasks, so retried + failed + skipped can exceed the
 * number of archived tasks that existed; never render that sum as a task
 * count that "has to add up".
 */
export function normalizeRetryAllResult(res: {
  retried?: number
  failed?: number
  skipped?: number
  by_type?: Record<string, number> | null
} | null | undefined): RetryAllResultSummary {
  return {
    retried: Number(res?.retried) || 0,
    failed: Number(res?.failed) || 0,
    skipped: Number(res?.skipped) || 0,
    byType: res?.by_type ?? {},
  }
}
