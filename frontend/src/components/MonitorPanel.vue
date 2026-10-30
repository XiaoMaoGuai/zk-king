<template>
  <div class="monitor">
    <div class="monitor-header">
      <div class="monitor-title">集群监控（四字命令）</div>
      <n-select
        v-model:value="selectedServer"
        :options="serverOptions"
        size="small"
        placeholder="选择 Server"
        class="server-select"
      />
      <n-space size="small">
        <n-button size="small" @click="run('ruok')">ruok</n-button>
        <n-button size="small" @click="run('stat')">stat</n-button>
        <n-button size="small" @click="run('srvr')">srvr</n-button>
        <n-button size="small" @click="run('mntr')">mntr</n-button>
        <n-button size="small" @click="run('cons')">cons</n-button>
        <n-button size="small" @click="run('wchs')">wchs</n-button>
        <n-button size="small" @click="run('envi')">envi</n-button>
      </n-space>
    </div>

    <div class="monitor-body">
      <!-- mntr 解析为表格 -->
      <template v-if="parsed.length > 0">
        <n-data-table :columns="mntrColumns" :data="parsed" :bordered="false" size="small" class="mntr-table" />
      </template>
      <!-- 其他命令原始输出 -->
      <pre v-else-if="rawOutput" class="raw-output">{{ rawOutput }}</pre>
      <div v-else class="monitor-empty">点击上方命令按钮查看 ZK Server 运行状态<br />（ruok=健康检查，mntr=详细指标，srvr=服务器状态，cons=客户端连接，wchs=watch 汇总）</div>

      <div class="watch-stream">
        <div class="watch-stream-title">实时事件流（Watch / 会话）</div>
        <div v-if="events.length === 0" class="watch-empty">暂无事件</div>
        <div v-for="(ev, i) in events" :key="i" class="watch-event">
          <n-tag size="tiny" :type="ev.kind === 'session' ? 'warning' : 'success'">{{ ev.time }}</n-tag>
          <span class="ev-path">{{ ev.kind === 'session' ? '[会话] ' + ev.state : ev.path }}</span>
          <n-tag size="tiny" type="info">{{ ev.kind === 'session' ? '' : ev.type }}</n-tag>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { NButton, NSpace, NSelect, NDataTable, NTag } from 'naive-ui'
import { FourLetterWord } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  connId: { type: String, required: true },
  conn: { type: Object, default: null }
})

const selectedServer = ref('')
const serverOptions = computed(() =>
  (props.conn?.servers || []).map(s => ({ label: s, value: s }))
)

const rawOutput = ref('')
const parsed = ref([])
const events = ref([])
let offs = []

const mntrColumns = [
  { title: '指标', key: 'key' },
  { title: '值', key: 'value' }
]

function run(cmd) {
  rawOutput.value = ''
  parsed.value = []
  if (!selectedServer.value && serverOptions.value.length) {
    selectedServer.value = serverOptions.value[0].value
  }
  FourLetterWord(props.connId, selectedServer.value, cmd)
    .then(text => {
      if (cmd === 'mntr') {
        parsed.value = String(text)
          .split('\n')
          .filter(l => l.includes('\t'))
          .map(l => {
            const [key, value] = l.split('\t')
            return { key: key.trim(), value: value.trim() }
          })
      } else if (cmd === 'ruok') {
        rawOutput.value = text.trim() === 'imok' ? 'imok —— 服务正常' : text
      } else {
        rawOutput.value = text
      }
    })
    .catch(e => {
      rawOutput.value = '命令执行失败: ' + e
    })
}

onMounted(() => {
  offs.push(EventsOn('zk-watch', ev => {
    events.value.unshift({ kind: 'watch', path: ev.path, type: ev.type, time: ev.time || new Date().toLocaleTimeString('zh-CN') })
    if (events.value.length > 200) events.value.length = 200
  }))
  offs.push(EventsOn('zk-session', ev => {
    events.value.unshift({ kind: 'session', state: ev.state, time: new Date().toLocaleTimeString('zh-CN') })
    if (events.value.length > 200) events.value.length = 200
  }))
})

onBeforeUnmount(() => {
  offs.forEach(fn => fn && fn())
})
</script>

<style scoped>
.monitor { flex: 1; display: flex; flex-direction: column; min-width: 0; padding: 12px 16px; }
.monitor-header { display: flex; align-items: center; gap: 12px; padding-bottom: 10px; border-bottom: 1px solid #2a2a30; }
.monitor-title { font-size: 15px; font-weight: 600; color: #63e2b7; white-space: nowrap; }
.server-select { width: 200px; }
.monitor-body { flex: 1; overflow: auto; padding-top: 12px; }
.mntr-table { margin-bottom: 16px; }
.raw-output {
  background: #1c1c21; border: 1px solid #2a2a30; border-radius: 8px;
  padding: 12px; font-size: 12px; line-height: 1.6; color: #ddd;
  white-space: pre-wrap; word-break: break-all;
}
.monitor-empty { color: #666; font-size: 14px; text-align: center; padding: 60px 0; line-height: 2; }
.watch-stream { margin-top: 20px; border-top: 1px solid #2a2a30; padding-top: 12px; }
.watch-stream-title { font-size: 13px; color: #aaa; margin-bottom: 8px; }
.watch-event { display: flex; align-items: center; gap: 8px; padding: 3px 0; font-size: 13px; }
.ev-path { color: #ccc; flex: 1; }
.watch-empty { color: #666; font-size: 13px; }
</style>
