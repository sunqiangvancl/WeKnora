import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

const tree = readFileSync(new URL('./KbFolderTree.vue', import.meta.url), 'utf8')
const batchBar = readFileSync(new URL('./DocumentBatchBar.vue', import.meta.url), 'utf8')
const cardView = readFileSync(new URL('./DocumentCardView.vue', import.meta.url), 'utf8')
const listView = readFileSync(new URL('./DocumentListView.vue', import.meta.url), 'utf8')
const kb = readFileSync(new URL('../KnowledgeBase.vue', import.meta.url), 'utf8')

// The root folder's own path IS the empty string, so a falsy "nothing is being
// renamed" sentinel matches it and leaves a stray rename input on the root row.
// Two independent guards keep that from coming back.
test('the rename sentinel cannot collide with the root folder path', () => {
  assert.match(tree, /const renamingPath = ref<string \| null>\(null\)/)
  assert.doesNotMatch(tree, /const renamingPath = ref\(''\)/)
  assert.match(tree, /row\.kind === 'folder' && renamingPath\.value === row\.path/)
})

test('only real folders expose a rename affordance', () => {
  assert.match(tree, /v-if="canEdit && row\.kind === 'folder'"/)
  assert.match(tree, /popup-menu-item/)
  assert.match(tree, /onFolderMenuRename/)
})

// Picking a folder is a small, reversible action, so it stays a popup: in the row
// menu it is another level of the menu that is already open, and in the batch bar
// it hangs off the button. Neither should escalate to a modal dialog.
test('the folder picker is a popup rather than a modal', () => {
  assert.match(cardView, /folderPickerItemId === item\.id/)
  assert.match(listView, /folderPickerItemId === item\.id/)
  assert.match(batchBar, /<t-popup[^>]*v-model:visible="folderPickerVisible"/)
  for (const source of [cardView, listView, batchBar]) {
    assert.doesNotMatch(source, /MoveToFolderDialog/)
  }
})

// The folder picker must render ahead of the normal action menu; otherwise
// moveMenuMode === 'normal' keeps the menu visible and clicks look dead.
test('the folder picker wins over the normal action menu', () => {
  for (const source of [cardView, listView]) {
    const pickerIdx = source.indexOf('v-if="folderPickerItemId === item.id"')
    const normalIdx = source.indexOf('v-else-if="moveMenuMode === \'normal\'"')
    assert.ok(pickerIdx >= 0)
    assert.ok(normalIdx >= 0)
    assert.ok(pickerIdx < normalIdx)
  }
})

test('folder navigation cancels stale card hover popovers', () => {
  assert.match(cardView, /if \(!cardElement\.isConnected/)
  assert.match(cardView, /watch\(\(\) => props\.items, dismissCardPopover\)/)
  assert.match(cardView, /const onOpenFolder = \(path: string\)[\s\S]*?dismissCardPopover\(\)[\s\S]*?emit\('open-folder', path\)/)
  assert.match(cardView, /@click\.stop="onOpenFolder\(item\.folder_path\)"/)
})

// The badge answers "how much does this folder hold", which is the whole subtree.
// The server already rolls it up into total_count; the row used to render the
// single-level value instead, so a parent with 226 docs underneath showed "2".
test('the folder count badge shows the subtree total', () => {
  assert.match(
    tree,
    /class="kb-folder-row__count"[\s\S]{0,300}?\{\{ row\.totalCount \}\}/,
  )
  assert.doesNotMatch(tree, /\{\{ row\.documentCount \}\}/)
  // The badge is the subtree total; the tooltip is still the only place that names
  // the single-level count, so it has to stay in the template.
  assert.match(tree, /direct: row\.documentCount/)
})

// The path used to be a chip under the name, shown only while filtering.
// With subtree browsing a single list spans many folders, so it becomes a column.
test('the list view has a real folder-path column', () => {
  assert.match(listView, /class="cell cell-path" role="columnheader"/)
  assert.match(listView, /class="cell cell-path" role="cell"/)
  // narrow variant must hide it rather than shift every grid-column index
  assert.match(listView, /\.cell-path\s*\{\s*display:\s*none/)
})

test('the header cells sit in the same order as the row cells', () => {
  const orderOf = (role: string) => listView
    .split('\n')
    .filter((line) => line.includes('role="' + role + '"'))
    .map((line) => (/class="cell (cell-[a-z]+)"/.exec(line) || ['', ''])[1])
  const expected = ['cell-check', 'cell-name', 'cell-path', 'cell-tags', 'cell-status', 'cell-actions']
  // path sits between name and tags in BOTH, so no track can drift one cell sideways
  assert.deepEqual(orderOf('columnheader'), expected)
  assert.deepEqual(orderOf('cell'), expected)
})

// The path column is a grid track, so the template and the cells cannot drift:
// an ungated six-track template hands a folderless knowledge base five cells in
// six tracks, an empty trailing track and a status column 88px too wide.
test('the six-track template is gated on the same condition as the path cells', () => {
  const six = '24px minmax(0, 1fr) minmax(140px, 18%) minmax(180px, 24%) 100px 28px'
  const five = '24px minmax(0, 1fr) minmax(180px, 24%) 100px 28px'
  // three grid definitions: five-track default, gated six-track, narrow override
  assert.equal((listView.match(/grid-template-columns/g) ?? []).length, 3)
  assert.equal(listView.split(six).length - 1, 1)
  assert.equal(listView.split(five).length - 1, 1)
  assert.match(
    listView,
    /\.doc-list-header\.has-path,\s*\.doc-list-row\.has-path\s*\{[^}]*grid-template-columns: 24px minmax\(0, 1fr\) minmax\(140px, 18%\) minmax\(180px, 24%\) 100px 28px;/)
  assert.match(
    listView,
    /\.doc-list-header,\s*\.doc-list-row\s*\{[^}]*grid-template-columns: 24px minmax\(0, 1fr\) minmax\(180px, 24%\) 100px 28px;/)
  // the class rides the same prop as both v-if cells, on the header and each row
  assert.match(listView, /class="doc-list-header"[^\n]*'has-path': showFolderPath/)
  assert.match(listView, /class="doc-list-row"[\s\S]{0,200}?'has-path': showFolderPath/)
})

test('the narrow override must out-specify the gated wide rule', () => {
  const narrow = listView.slice(listView.indexOf('@container doc-card-area (max-width: 720px)'))
  assert.ok(narrow.length > 0)
  // @container adds no specificity, so a bare (0,1,0) narrow rule loses to
  // `.doc-list-header.has-path` (0,2,0) and keeps six tracks in a 720px panel:
  // the narrow template has to restate the gated pair as well as the plain one.
  assert.match(
    narrow,
    /\.doc-list-header,\s*\.doc-list-row,\s*\.doc-list-header\.has-path,\s*\.doc-list-row\.has-path\s*\{[^}]*grid-template-columns: 24px minmax\(0, 1fr\) 100px 28px;/)
  // the path column is dropped, not re-indexed, so these indices stay verbatim
  assert.match(narrow, /\.cell-path\s*\{\s*display:\s*none/)
  assert.match(narrow, /\.doc-list-row \.cell-tags\s*\{\s*grid-column: 2;\s*grid-row: 2;\s*padding-left: 44px;\s*\}/)
  assert.match(narrow, /\.cell-status\s*\{\s*grid-column: 3;\s*grid-row: 1;\s*\}/)
  assert.match(narrow, /\.cell-actions\s*\{\s*grid-column: 4;\s*grid-row: 1;\s*\}/)
})

// One prop used to drive both views, so widening it to make the column always
// available silently widened the card too, where the chip is the v-if arm of a
// v-if/v-else pair and every filed card lost its updated timestamp. The gates are
// two separate computeds now, one per view.
test('the path column and the card chip get separate gates', () => {
  const attrsOf = (component: string) => {
    const start = kb.indexOf('<' + component)
    return kb.slice(start, kb.indexOf('/>', start))
  }
  assert.match(kb, /const showDocumentFolderColumn = computed\(\(\) => hasFolders\.value\);/)
  assert.match(kb, /const showDocumentFolderPath = computed\(\(\) => hasFolders\.value && isFiltering\.value\);/)
  assert.match(attrsOf('DocumentListView'), /:show-folder-path="showDocumentFolderColumn"/)
  assert.match(attrsOf('DocumentCardView'), /:show-folder-path="showDocumentFolderPath"/)
  // the chip competes with the timestamp for one slot, so it stays filter-gated
  assert.match(cardView, /v-if="showFolderPath && item\.folder_path"[\s\S]{0,400}?<span v-else class="card-time"/)
})
