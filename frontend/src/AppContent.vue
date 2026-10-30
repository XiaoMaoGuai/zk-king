<template>
  <div class="app">
    <header class="header">
      <div class="logo">ZK-King</div>
      <div class="actions">
        <n-button size="small" :disabled="!currentId" @click="showMonitor = !showMonitor">
          {{ showMonitor ? '返回节点浏览' : '集群监控' }}
        </n-button>
        <n-button size="small" type="primary" @click="openConnModal">＋ 新建连接</n-button>
      </div>
    </header>

    <main class="main">
      <aside class="sidebar">
        <div class="sidebar-title">连接</div>
        <div class="conn-list">
          <div
            v-for="c in connections"
            :key="c.id"
            class="conn-card"
            :class="{ active: currentId === c.id }"
            @click="switchConn(c.id)"
          >
            <div class="conn-name">{{ c.name || c.hosts }}</div>
            <div class="conn-hosts">{{ c.hosts }}{{ c.chroot ? ' /' + c.chroot : '' }}</div>
            <div class="conn-status">
              <span class="dot" :class="c.connected ? 'on' : 'off'"></span>
              {{ c.connected ? '已连接' : '断开' }}
            </div>
            <n-button text type="error" size="tiny" class="del" @click.stop="disconnect(c.id)">断开</n-button>
          </div>
          <div v-if="connections.length === 0" class="conn-empty">
            暂无连接<br />点右上角「新建连接」添加
          </div>
        </div>
      </aside>

      <section class="content">
        <template v-if="currentId">
          <znode-explorer
            v-if="!showMonitor"
            :conn-id="currentId"
            :conn-name="currentName"
          />
          <monitor-panel v-else :conn-id="currentId" :conn="currentConnInfo" />
        </template>
        <div v-else class="empty-state">
          <div class="empty-title">ZK-King</div>
          <div class="empty-sub">请选择左侧连接，或新建一个 Zookeeper 连接开始浏览</div>
        </div>
      </section>
    </main>
  </div>

  <n-modal v-model:show="showConnModal" preset="card" title="新建连接" style="width: 460px">
    <n-form :model="form" label-placement="top">
      <n-form-item label="名称（可选）">
        <n-input v-model:value="form.name" placeholder="例如：测试环境 ZK" />
      </n-form-item>
      <n-form-item label="连接串（host:port，多个用逗号分隔）">
        <n-input v-model:value="form.hosts" placeholder="192.168.1.10:2181,192.168.1.11:2181" />
      </n-form-item>
      <n-form-item label="chroot（可选，如 /dubbo）">
        <n-input v-model:value="form.chroot" placeholder="/" />
      </n-form-item>
      <n-form-item label="digest 认证（可选，user:pass）">
        <n-input v-model:value="form.auth" placeholder="admin:123456" />
      </n-form-item>
      <n-form-item label="连接超时（秒）">
        <n-input-number v-model:value="form.timeout" :min="3" :max="60" />
      </n-form-item>
      <n-button type="primary" block :loading="connecting" @click="doConnect">连接</n-button>
    </n-form>
  </n-modal>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  NButton, NModal,
  NForm, NFormItem, NInput, NInputNumber, useMessage
} from 'naive-ui'
import ZnodeExplorer from './components/ZnodeExplorer.vue'
import MonitorPanel from './components/MonitorPanel.vue'
import { ConnectZk, DisconnectZk, GetConnections } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

const connections = ref([])
const currentId = ref('')
const showConnModal = ref(false)
const showMonitor = ref(false)
const connecting = ref(false)
const form = ref({ name: '', hosts: '', chroot: '', auth: '', timeout: 10 })
const message = useMessage()

const currentConnInfo = computed(() => connections.value.find(c => c.id === currentId.value))
const currentName = computed(() => currentConnInfo.value?.name || currentConnInfo.value?.hosts || '')

async function refresh() {
  connections.value = (await GetConnections()) || []
}
onMounted(refresh)

function openConnModal() {
  showConnModal.value = true
}

function switchConn(id) {
  currentId.value = id
  showMonitor.value = false
}

async function doConnect() {
  if (!form.value.hosts) {
    message.warning('请填写连接串')
    return
  }
  connecting.value = true
  try {
    const info = await ConnectZk(form.value.name, form.value.hosts, form.value.chroot, form.value.auth, form.value.timeout)
    message.success('连接成功')
    showConnModal.value = false
    await refresh()
    currentId.value = info.id
  } catch (e) {
    message.error(typeof e === 'string' ? e : String(e))
  } finally {
    connecting.value = false
  }
}

async function disconnect(id) {
  try {
    await DisconnectZk(id)
  } catch (e) {
    /* 忽略 */
  }
  if (currentId.value === id) {
    currentId.value = ''
    showMonitor.value = false
  }
  await refresh()
}

// 会话状态事件
EventsOn('zk-session', (ev) => {
  message.info(`连接 ${ev.id} 会话状态: ${ev.state}`, { duration: 3000 })
})
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body, #app { height: 100%; }
body {
  font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  background: #18181c;
  color: #eee;
}
.app { display: flex; flex-direction: column; height: 100vh; }

.header {
  display: flex; align-items: center; justify-content: space-between;
  height: 48px; padding: 0 16px;
  background: #1f1f25; border-bottom: 1px solid #2a2a30;
}
.logo { font-size: 18px; font-weight: 700; color: #63e2b7; letter-spacing: 1px; }
.actions { display: flex; gap: 8px; }

.main { flex: 1; display: flex; min-height: 0; }

.sidebar {
  width: 260px; min-width: 260px;
  border-right: 1px solid #2a2a30;
  display: flex; flex-direction: column;
  background: #1c1c21;
}
.sidebar-title { padding: 12px 16px 6px; font-size: 12px; color: #888; }
.conn-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; }
.conn-card {
  position: relative;
  background: #24242b; border: 1px solid #2e2e36; border-radius: 8px;
  padding: 10px 12px; margin-bottom: 8px; cursor: pointer;
  transition: border-color .2s, background .2s;
}
.conn-card:hover { border-color: #3e3e48; }
.conn-card.active { border-color: #63e2b7; background: #26312e; }
.conn-name { font-size: 14px; font-weight: 600; margin-bottom: 4px; padding-right: 40px; }
.conn-hosts { font-size: 12px; color: #999; margin-bottom: 6px; word-break: break-all; }
.conn-status { font-size: 12px; color: #aaa; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
.dot.on { background: #63e2b7; box-shadow: 0 0 6px #63e2b7; }
.dot.off { background: #f56c6c; }
.del { position: absolute; top: 10px; right: 10px; }
.conn-empty { text-align: center; color: #777; font-size: 13px; padding: 40px 0; line-height: 1.8; }

.content { flex: 1; display: flex; min-width: 0; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; color: #666; }
.empty-title { font-size: 32px; font-weight: 700; color: #3a3a42; margin-bottom: 12px; }
.empty-sub { font-size: 14px; }
</style>
