<template>
  <div class="explorer">
    <!-- 左：节点树 -->
    <div class="tree-pane">
      <div class="pane-header">
        <span class="pane-title">{{ connName }}</span>
        <n-button size="tiny" quaternary @click="reloadTree">刷新</n-button>
      </div>
      <n-tree
        :data="treeData"
        :on-load="handleLoad"
        :selected-keys="selectedKeys"
        :default-expand-all="false"
        block-node
        selectable
        @update:selected-keys="onSelect"
        class="tree"
      />
    </div>

    <!-- 右：节点详情 -->
    <div class="detail-pane">
      <template v-if="selectedPath">
        <div class="detail-header">
          <span class="path">{{ selectedPath }}</span>
          <n-space size="small">
            <n-button size="tiny" type="primary" ghost @click="openCreateModal">新建子节点</n-button>
            <n-button size="tiny" type="warning" ghost @click="copyPath">复制路径</n-button>
            <n-button size="tiny" type="error" ghost @click="doDelete">删除节点</n-button>
          </n-space>
        </div>

        <n-tabs v-model:value="activeTab" type="line" size="small" class="detail-tabs">
          <!-- 数据 -->
          <n-tab-pane name="data" tab="数据">
            <div v-if="detail" class="data-box">
              <n-input
                v-model:value="editableData"
                type="textarea"
                :autosize="{ minRows: 8, maxRows: 20 }"
                placeholder="节点数据（文本）"
              />
              <div class="data-actions">
                <n-button size="tiny" type="primary" :loading="saving" @click="saveData">保存数据</n-button>
                <n-button size="tiny" quaternary @click="formatJson">格式化 JSON</n-button>
                <n-tag size="small" type="info">dataLength: {{ detail.stat.dataLength }}</n-tag>
              </div>
            </div>
          </n-tab-pane>

          <!-- Stat -->
          <n-tab-pane name="stat" tab="Stat">
            <n-descriptions v-if="detail" :column="2" bordered size="small" label-placement="left" class="stat-desc">
              <n-descriptions-item label="Czxid">{{ detail.stat.czxid }}</n-descriptions-item>
              <n-descriptions-item label="Mzxid">{{ detail.stat.mzxid }}</n-descriptions-item>
              <n-descriptions-item label="Ctime">{{ fmtTime(detail.stat.ctime) }}</n-descriptions-item>
              <n-descriptions-item label="Mtime">{{ fmtTime(detail.stat.mtime) }}</n-descriptions-item>
              <n-descriptions-item label="Version">{{ detail.stat.version }}</n-descriptions-item>
              <n-descriptions-item label="Cversion">{{ detail.stat.cversion }}</n-descriptions-item>
              <n-descriptions-item label="Aversion">{{ detail.stat.aversion }}</n-descriptions-item>
              <n-descriptions-item label="EphemeralOwner">
                <n-tag v-if="detail.stat.ephemeralOwner !== 0" size="small" type="warning">临时节点</n-tag>
                <span v-else>0（持久节点）</span>
              </n-descriptions-item>
              <n-descriptions-item label="DataLength">{{ detail.stat.dataLength }}</n-descriptions-item>
              <n-descriptions-item label="NumChildren">{{ detail.stat.numChildren }}</n-descriptions-item>
              <n-descriptions-item label="Pzxid">{{ detail.stat.pzxid }}</n-descriptions-item>
            </n-descriptions>
          </n-tab-pane>

          <!-- ACL -->
          <n-tab-pane name="acl" tab="ACL">
            <template v-if="detail">
              <n-data-table
                :columns="aclColumns"
                :data="detail.acl"
                :bordered="false"
                size="small"
                class="acl-table"
              />
              <n-button size="tiny" type="primary" ghost class="acl-edit-btn" @click="openAclModal">编辑 ACL</n-button>
            </template>
          </n-tab-pane>

          <!-- 监听 -->
          <n-tab-pane name="watch" tab="监听器">
            <div class="watch-add">
              <n-input v-model:value="watchPath" placeholder="输入要监听的节点路径，如 /dubbo" size="small" />
              <n-button size="small" type="primary" @click="addWatch">添加监听</n-button>
            </div>
            <n-data-table
              :columns="watchColumns"
              :data="watchList"
              :bordered="false"
              size="small"
              class="acl-table"
            />
            <div class="watch-events">
              <div class="watch-events-title">实时事件（Watch 触发推送）</div>
              <div v-if="watchEvents.length === 0" class="watch-empty">暂无事件，添加监听后等待节点变化</div>
              <div v-for="(ev, i) in watchEvents" :key="i" class="watch-event">
                <n-tag size="tiny" type="info">{{ ev.time }}</n-tag>
                <span class="ev-path">{{ ev.path }}</span>
                <n-tag size="tiny" :type="ev.type.includes('Deleted') ? 'error' : 'success'">{{ ev.type }}</n-tag>
              </div>
            </div>
          </n-tab-pane>
        </n-tabs>
      </template>
      <div v-else class="detail-empty">← 在左侧选择一个节点查看详情</div>
    </div>

    <!-- 新建子节点 -->
    <n-modal v-model:show="showCreateModal" preset="dialog" title="新建子节点" :positive-text="'创建'" :negative-text="'取消'"
      @positive-click="doCreate" @negative-click="showCreateModal = false">
      <n-form :model="createForm" label-placement="top" size="small" class="modal-form">
        <n-form-item label="父路径"><n-input :value="selectedPath" disabled /></n-form-item>
        <n-form-item label="节点名称"><n-input v-model:value="createForm.name" placeholder="子节点名，如 config" /></n-form-item>
        <n-form-item label="节点类型">
          <n-select v-model:value="createForm.type" :options="nodeTypeOptions" />
        </n-form-item>
        <n-form-item label="节点数据"><n-input v-model:value="createForm.data" type="textarea" :autosize="{ minRows: 3 }" /></n-form-item>
      </n-form>
    </n-modal>

    <!-- 删除节点确认 -->
    <n-modal v-model:show="showDeleteModal" preset="dialog" title="删除节点" :positive-text="'删除'" :negative-text="'取消'"
      @positive-click="confirmDelete" @negative-click="showDeleteModal = false">
      <div class="delete-tip">
        确认删除节点 <span class="del-path">{{ selectedPath }}</span> ？
        <n-checkbox v-model:checked="deleteRecursiveFlag" class="del-recursive">
          递归删除所有子节点（{{ detail?.stat?.numChildren || 0 }} 个直接子节点）
        </n-checkbox>
      </div>
    </n-modal>

    <!-- 编辑 ACL -->
    <n-modal v-model:show="showAclModal" preset="dialog" title="编辑 ACL（每行一条：scheme:id:perms）"
      :positive-text="'保存'" :negative-text="'取消'"
      @positive-click="saveAcl" @negative-click="showAclModal = false">
      <n-input v-model:value="aclText" type="textarea" :autosize="{ minRows: 6 }"
        placeholder="world:anyone:cdrwa&#10;digest:admin:hash:cdrwa" />
    </n-modal>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onBeforeUnmount, h } from 'vue'
import {
  NTree, NButton, NSpace, NTabs, NTabPane, NInput, NTag,
  NDataTable, NDescriptions, NDescriptionsItem, NModal, NForm, NFormItem, NSelect, NCheckbox, useMessage
} from 'naive-ui'
import {
  GetChildren, GetNode, CreateNode, UpdateNode, DeleteNode, DeleteNodeRecursive,
  SetNodeACL, AddWatch, GetWatches, RemoveWatch
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  connId: { type: String, required: true },
  connName: { type: String, default: '' }
})

const message = useMessage()
const treeData = ref([])
const selectedKeys = ref([])
const selectedPath = ref('')
const detail = ref(null)
const editableData = ref('')
const activeTab = ref('data')
const saving = ref(false)

const showCreateModal = ref(false)
const createForm = ref({ name: '', type: 0, data: '' })
const nodeTypeOptions = [
  { label: '持久节点', value: 0 },
  { label: '临时节点（Ephemeral）', value: 1 },
  { label: '持久顺序节点', value: 2 }
]

const showAclModal = ref(false)
const aclText = ref('')

const showDeleteModal = ref(false)
const deleteRecursiveFlag = ref(false)

const watchPath = ref('')
const watchList = ref([])
const watchEvents = ref([])
let offWatchEvent = null

const aclColumns = [
  { title: 'Scheme', key: 'scheme' },
  { title: 'ID', key: 'id' },
  { title: '权限', key: 'perms' },
  { title: '原始值', key: 'raw' }
]

const watchColumns = [
  { title: '路径', key: 'path' },
  {
    title: '操作', key: 'op',
    render: (row) => {
      return h(NButton, {
        size: 'tiny', type: 'error', ghost: true,
        onClick: () => doRemoveWatch(row.path)
      }, { default: () => '移除' })
    }
  }
]

// 树的节点构造与懒加载
function makeNode(parent, name) {
  const full = parent === '/' ? '/' + name : parent + '/' + name
  return { label: name, key: full, isLeaf: false }
}

async function loadRoot() {
  try {
    const children = await GetChildren(props.connId, '/')
    treeData.value = [{
      label: '/',
      key: '/',
      isLeaf: children.length === 0,
      children: children.map(c => makeNode('/', c))
    }]
  } catch (e) {
    message.error(String(e))
  }
}

const handleLoad = (node) => new Promise((resolve) => {
  GetChildren(props.connId, node.key)
    .then(children => {
      node.children = children.map(c => makeNode(node.key, c))
      node.isLeaf = children.length === 0
      resolve()
    })
    .catch((e) => {
      node.isLeaf = true
      message.error('加载子节点失败: ' + String(e))
      resolve()
    })
})

async function onSelect(keys) {
  if (!keys || keys.length === 0) return
  selectedPath.value = keys[0]
  await loadDetail(selectedPath.value)
}

async function loadDetail(path) {
  try {
    detail.value = await GetNode(props.connId, path)
    editableData.value = detail.value.data
  } catch (e) {
    message.error(String(e))
  }
}

function reloadTree() {
  loadRoot()
  if (selectedPath.value) loadDetail(selectedPath.value)
}

function copyPath() {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(selectedPath.value)
    message.success('已复制路径: ' + selectedPath.value)
  }
}

async function saveData() {
  saving.value = true
  try {
    await UpdateNode(props.connId, selectedPath.value, editableData.value)
    message.success('数据已保存')
    await loadDetail(selectedPath.value)
  } catch (e) {
    message.error(String(e))
  } finally {
    saving.value = false
  }
}

function openCreateModal() {
  createForm.value = { name: '', type: 0, data: '' }
  showCreateModal.value = true
}

async function doCreate() {
  if (!createForm.value.name) {
    message.warning('请输入节点名称')
    return false
  }
  try {
    await CreateNode(props.connId, selectedPath.value, createForm.value.name, createForm.value.data, createForm.value.type)
    message.success('创建成功')
    showCreateModal.value = false
    await reloadTree()
    await loadDetail(selectedPath.value)
  } catch (e) {
    message.error(String(e))
  }
  return true
}

async function doDelete() {
  if (selectedPath.value === '/') {
    message.warning('不能删除根节点')
    return
  }
  deleteRecursiveFlag.value = false
  showDeleteModal.value = true
}

async function confirmDelete() {
  try {
    if (deleteRecursiveFlag.value) {
      await DeleteNodeRecursive(props.connId, selectedPath.value)
    } else {
      await DeleteNode(props.connId, selectedPath.value)
    }
    message.success('已删除: ' + selectedPath.value)
    showDeleteModal.value = false
    selectedPath.value = ''
    selectedKeys.value = []
    detail.value = null
    await reloadTree()
  } catch (e) {
    message.error(String(e))
    return false
  }
  return true
}

function formatJson() {
  const text = editableData.value.trim()
  if (!text) return
  try {
    editableData.value = JSON.stringify(JSON.parse(text), null, 2)
    message.success('已格式化')
  } catch (e) {
    message.error('不是合法的 JSON: ' + e.message)
  }
}

function openAclModal() {
  aclText.value = (detail.value?.acl || []).map(a => `${a.scheme}:${a.id}:${a.perms}`).join('\n')
  showAclModal.value = true
}

async function saveAcl() {
  try {
    await SetNodeACL(props.connId, selectedPath.value, aclText.value)
    message.success('ACL 已更新')
    showAclModal.value = false
    await loadDetail(selectedPath.value)
  } catch (e) {
    message.error(String(e))
  }
  return true
}

async function addWatch() {
  const p = watchPath.value.trim()
  if (!p) {
    message.warning('请输入节点路径')
    return
  }
  try {
    await AddWatch(props.connId, p)
    message.success('已添加监听: ' + p)
    watchPath.value = ''
    await refreshWatches()
  } catch (e) {
    message.error(String(e))
  }
}

async function refreshWatches() {
  watchList.value = ((await GetWatches(props.connId)) || []).map(p => ({ path: p }))
}

async function doRemoveWatch(path) {
  await RemoveWatch(props.connId, path)
  await refreshWatches()
}

function fmtTime(ms) {
  if (!ms) return '-'
  return new Date(ms).toLocaleString('zh-CN')
}

watch(() => props.connId, async () => {
  detail.value = null
  selectedPath.value = ''
  selectedKeys.value = []
  watchEvents.value = []
  await refreshWatches()
  await loadRoot()
})

onMounted(async () => {
  await refreshWatches()
  await loadRoot()
  offWatchEvent = EventsOn('zk-watch', (ev) => {
    if (ev.id !== props.connId) return
    watchEvents.value.unshift({
      path: ev.path,
      type: ev.type,
      time: ev.time || new Date().toLocaleTimeString('zh-CN')
    })
    if (watchEvents.value.length > 100) watchEvents.value.length = 100
  })
})

onBeforeUnmount(() => {
  if (offWatchEvent) offWatchEvent()
})
</script>

<style scoped>
.explorer { flex: 1; display: flex; min-width: 0; min-height: 0; }

.tree-pane {
  width: 320px; min-width: 320px;
  border-right: 1px solid #2a2a30;
  display: flex; flex-direction: column;
  background: #1c1c21;
}
.pane-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 12px; border-bottom: 1px solid #2a2a30;
}
.pane-title { font-size: 13px; color: #ccc; font-weight: 600; }
.tree { flex: 1; overflow: auto; padding: 8px; }

.detail-pane { flex: 1; display: flex; flex-direction: column; min-width: 0; padding: 12px 16px; }
.detail-header {
  display: flex; justify-content: space-between; align-items: center;
  padding-bottom: 10px; border-bottom: 1px solid #2a2a30; margin-bottom: 8px;
}
.path { font-size: 15px; font-weight: 600; color: #63e2b7; word-break: break-all; }
.detail-tabs { flex: 1; min-height: 0; }
.data-box { display: flex; flex-direction: column; gap: 8px; }
.data-actions { display: flex; align-items: center; gap: 12px; }
.stat-desc { margin-top: 8px; }
.acl-table { margin-top: 8px; }
.acl-edit-btn { margin-top: 12px; }
.watch-add { display: flex; gap: 8px; margin-bottom: 12px; }
.watch-events { margin-top: 16px; }
.watch-events-title { font-size: 13px; color: #aaa; margin-bottom: 8px; }
.watch-event { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px; }
.ev-path { color: #ccc; flex: 1; }
.watch-empty { color: #666; font-size: 13px; padding: 12px 0; }
.detail-empty { flex: 1; display: flex; align-items: center; justify-content: center; color: #666; }
.modal-form { margin-top: 8px; }
.delete-tip { font-size: 13px; color: #ddd; line-height: 2; }
.del-path { color: #f56c6c; word-break: break-all; }
.del-recursive { margin-top: 8px; }
</style>
