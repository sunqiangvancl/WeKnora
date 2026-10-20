import assert from 'node:assert/strict'
import test from 'node:test'

import type { RuntimeTask } from '@/api/system'
import {
  RUNTIME_TASK_FAILURE_CODES,
  runtimeTaskFailureDispositionKey,
  runtimeTaskFailureReasonKey,
  runtimeTaskMetaRows,
  runtimeTaskModelRoleKey,
  runtimeTaskModelRows,
} from './runtimeTaskDisplay.ts'

function task(overrides: Partial<RuntimeTask>): RuntimeTask {
  return {
    id: 'task-1',
    queue: 'default',
    type: 'document:process',
    state: 'archived',
    allowed_actions: [],
    retried: 1,
    max_retry: 3,
    ...overrides,
  }
}

test('every ID row carries the resolved name when the backend supplies one', () => {
  const rows = runtimeTaskMetaRows(
    task({
      knowledge_base_id: 'kb-1',
      knowledge_base_name: '盘古SRM产品1.69',
      knowledge_id: 'doc-1',
      knowledge_name: '采购需求说明.pdf',
      data_source_id: 'ds-1',
      data_source_name: '飞书知识库同步',
      tenant_id: 10000,
      tenant_name: '服务商',
      source_kb_id: 'kb-2',
      source_kb_name: '大北农-饲料库',
      target_kb_id: 'kb-3',
      target_kb_name: '新建知识库',
    }),
  )
  const byKey = Object.fromEntries(rows.map((row) => [row.key, row]))

  assert.equal(byKey.kb.name, '盘古SRM产品1.69')
  assert.equal(byKey.kb.value, 'kb-1')
  assert.equal(byKey.kb.labelKey, 'system.globalSettings.runtime.tasks.knowledgeBaseLabel')
  assert.equal(byKey.knowledge.name, '采购需求说明.pdf')
  assert.equal(byKey.datasource.name, '飞书知识库同步')
  assert.equal(byKey.tenant.name, '服务商')
  assert.equal(byKey['source-kb'].name, '大北农-饲料库')
  assert.equal(byKey['target-kb'].name, '新建知识库')
  // Rows without a resolver keep their name empty so the SFC shows the ID alone.
  assert.equal(byKey.task, undefined)
})

test('the document row falls back to the upload file name when the row is gone', () => {
  const rows = runtimeTaskMetaRows(
    task({ knowledge_id: 'doc-1', knowledge_file_name: '要求.xlsx' }),
  )
  assert.equal(rows.find((row) => row.key === 'knowledge')?.name, '要求.xlsx')
})

test('kb:clone shows knowledge base names on the generic source/target slots', () => {
  const rows = runtimeTaskMetaRows(
    task({
      type: 'kb:clone',
      source_id: 'kb-1',
      target_id: 'kb-2',
      source_kb_name: '盘古SRM产品1.69',
      target_kb_name: '大北农-饲料库',
    }),
  )
  const byKey = Object.fromEntries(rows.map((row) => [row.key, row]))
  assert.equal(byKey.source.name, '盘古SRM产品1.69')
  assert.equal(byKey.target.name, '大北农-饲料库')
})

test('generic source/target rows never borrow a knowledge base name', () => {
  const rows = runtimeTaskMetaRows(
    task({
      type: 'knowledge:move',
      source_id: 'source-1',
      target_id: 'target-1',
      source_kb_name: '盘古SRM产品1.69',
      target_kb_name: '大北农-饲料库',
    }),
  )
  const byKey = Object.fromEntries(rows.map((row) => [row.key, row]))
  assert.equal(byKey.source.name, undefined)
  assert.equal(byKey.target.name, undefined)
})

test('model rows expose both the role label key and the raw model ID', () => {
  const rows = runtimeTaskModelRows(
    task({
      models: [
        { role: 'summary', id: 'model-summary', name: 'DeepSeek Flash' },
        { role: 'embedding', id: 'model-embedding' },
        { role: 'nonsense', id: 'model-other' },
      ],
    }),
  )
  assert.equal(rows.length, 3)
  assert.deepEqual(
    rows.map((row) => [row.roleKey, row.id, row.name]),
    [
      ['system.globalSettings.runtime.tasks.modelRoles.summary', 'model-summary', 'DeepSeek Flash'],
      ['system.globalSettings.runtime.tasks.modelRoles.embedding', 'model-embedding', undefined],
      ['system.globalSettings.runtime.tasks.modelRoles.model', 'model-other', undefined],
    ],
  )
  assert.equal(runtimeTaskModelRoleKey('chat'), 'system.globalSettings.runtime.tasks.modelRoles.chat')
})

test('failure codes map to a localized reason key and fall back for unknown codes', () => {
  assert.equal(
    runtimeTaskFailureReasonKey('MODEL_RATE_LIMITED'),
    'system.globalSettings.runtime.tasks.failureCodes.MODEL_RATE_LIMITED',
  )
  for (const code of RUNTIME_TASK_FAILURE_CODES) {
    assert.equal(
      runtimeTaskFailureReasonKey(code),
      `system.globalSettings.runtime.tasks.failureCodes.${code}`,
      code,
    )
  }
  assert.equal(
    runtimeTaskFailureReasonKey('SOMETHING_NEW'),
    'system.globalSettings.runtime.tasks.failureCodes.UNKNOWN',
  )
})

test('failure dispositions map to hints and an absent disposition stays absent', () => {
  assert.equal(
    runtimeTaskFailureDispositionKey('retryable'),
    'system.globalSettings.runtime.tasks.failureDispositions.retryable',
  )
  assert.equal(
    runtimeTaskFailureDispositionKey('strange'),
    'system.globalSettings.runtime.tasks.failureDispositions.unknown',
  )
  assert.equal(runtimeTaskFailureDispositionKey(undefined), null)
})