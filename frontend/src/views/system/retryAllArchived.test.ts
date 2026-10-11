import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DETERMINISTIC_RETRY_ALL_TYPE,
  defaultRetryAllSelection,
  normalizeRetryAllFacets,
  normalizeRetryAllResult,
} from './retryAllArchived.ts'

test('facets are sorted by count desc, then by type for a deterministic order', () => {
  assert.deepEqual(
    normalizeRetryAllFacets({
      'document:process': 2,
      'summary:generation': 9,
      'datatable:summary': 2,
    }),
    [
      { type: 'summary:generation', count: 9 },
      { type: 'datatable:summary', count: 2 },
      { type: 'document:process', count: 2 },
    ],
  )
})

test('normalizeRetryAllFacets tolerates null and undefined', () => {
  assert.deepEqual(normalizeRetryAllFacets(undefined), [])
  assert.deepEqual(normalizeRetryAllFacets(null), [])
  assert.deepEqual(normalizeRetryAllFacets({}), [])
})

test('datatable:summary stays unchecked by default, everything else is selected', () => {
  assert.equal(DETERMINISTIC_RETRY_ALL_TYPE, 'datatable:summary')
  const facets = normalizeRetryAllFacets({
    'datatable:summary': 40,
    'document:process': 3,
    'wiki:ingest': 1,
  })
  assert.deepEqual(defaultRetryAllSelection(facets), ['document:process', 'wiki:ingest'])
})

test('default selection never invents a type that is not in the facets', () => {
  assert.deepEqual(defaultRetryAllSelection([]), [])
  assert.deepEqual(
    defaultRetryAllSelection(normalizeRetryAllFacets({ 'kb:delete': 5 })),
    ['kb:delete'],
  )
})

test('retry_all result tolerates a null by_type and missing counters', () => {
  assert.deepEqual(normalizeRetryAllResult(null), { retried: 0, failed: 0, skipped: 0, byType: {} })
  assert.deepEqual(
    normalizeRetryAllResult({ retried: 3, failed: 5, skipped: 1, by_type: null }),
    { retried: 3, failed: 5, skipped: 1, byType: {} },
  )
})

test('retry_all result passes through a present by_type map', () => {
  assert.deepEqual(
    normalizeRetryAllResult({
      retried: 1,
      failed: 0,
      skipped: 2,
      by_type: { 'document:process': 1 },
    }).byType,
    { 'document:process': 1 },
  )
})
