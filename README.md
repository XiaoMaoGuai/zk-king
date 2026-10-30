# ZK-King

一个现代、轻量、跨平台的 **Zookeeper 可视化客户端**，参考 [ES-King](https://github.com/Bronya0/ES-King) 的技术栈与设计风格打造。

- **技术栈**：Wails v2（Go 后端）+ Vue 3 + Naive UI（深色主题）
- **通信方式**：Go 侧通过 `go-zookeeper` 客户端直连 ZK（TCP 协议，兼容 ZK 3.4 ~ 3.8）
- **安装包**：跨平台（Windows / macOS / Linux），体积小

## 功能清单

### 连接管理
- 多集群连接卡片（名称、连接串、chroot、digest 认证、超时配置）
- 连接状态指示灯：在线绿点 / 断开红点，一眼识别会话健康度
- 会话状态实时推送（连接 / 断开 / 过期），通过 Wails 事件 `zk-session` 推送到前端
- 一键切换 / 断开连接，断开后自动清理该会话上的所有 Watch

### Znode 节点浏览与操作
- 树形懒加载浏览全部节点，支持任意层级，展开时按需加载子节点
- 一键刷新节点树，实时同步 ZK 最新结构
- 节点数据查看与在线编辑（文本模式），支持一键 **JSON 格式化**，实时显示 `dataLength`
- 节点创建：持久 / 临时（Ephemeral）/ 持久顺序 三类
- 节点删除：
  - 普通删除（空节点）
  - **递归删除**：勾选后自动先删子孙再删自身，可删除非空节点，弹窗显示直接子节点数量
- 复制节点路径
- 节点 Stat 元数据全量展示（czxid / mzxid / version / ephemeralOwner / dataLength / numChildren 等），临时节点自动高亮标识

### ACL 权限管理
- 节点 ACL 查看（scheme / id / 权限位 cdrwa / 原始值）
- 在线编辑 ACL（支持 world / auth / digest / ip 方案，多行输入，每行一条 `scheme:id:perms`）

### 监听器（Watch）
- 按路径注册 / 移除 Watch，查看当前会话已注册的 Watch 列表
- 节点变更事件（Created / Deleted / DataChanged / ChildrenChanged）通过 `zk-watch` 事件实时推送展示

### 集群监控（四字命令）
- ruok（健康检查）、stat / srvr（服务器状态）、mntr（详细指标，自动解析为表格）、cons（客户端连接）、wchs（watch 汇总）、envi（环境信息）
- 支持按 Server 选择目标节点（多 Server 集群逐台查看）
- 实时事件流：聚合展示当前连接的 Watch 事件与会话状态变化

## 目录结构

```
zk-king/
├── main.go            # Wails 入口（窗口配置、资源嵌入）
├── app.go             # ZK 服务层：连接/节点/ACL/Watch/四字命令
├── wails.json         # Wails 项目配置
├── go.mod
└── frontend/
    ├── package.json   # Vue3 + Naive UI
    ├── vite.config.js
    ├── index.html
    ├── wailsjs/       # Wails 自动生成的前端绑定（Go 方法的 JS 入口）
    └── src/
        ├── main.js
        ├── App.vue                # 根组件（Naive UI 主题 / 消息容器）
        ├── AppContent.vue         # 布局 + 连接管理
        └── components/
            ├── ZnodeExplorer.vue  # 节点树 + 数据/Stat/ACL/监听
            └── MonitorPanel.vue   # 四字命令监控 + 事件流
```

> 注意：`wailsjs/` 生成在 `frontend/wailsjs/`（而非 `src/` 下）。
> `src/components/` 中的组件需用 `../../wailsjs/...` 引用，`src/` 下的文件用 `../wailsjs/...`。

## 环境要求

- Go 1.21+
- Node.js 20.19+ / npm（Vite 7 要求）
- Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

前端关键依赖（对齐 ES-King 验证过的组合）：Vue 3.5.8 + Naive UI 2.39.0 + Vite 7.1.5，全量注册 Naive UI 组件。

## 运行与构建

```bash
cd zk-king

# 开发模式（热更新，前端改动即时生效）
wails dev

# 构建桌面应用
wails build

# 指定平台（需在对应系统上执行，Wails v2 不支持 macOS 交叉编译 Windows）
wails build -platform windows/amd64 -webview2 download -o ZK-King.exe
wails build -platform darwin/universal -o ZK-King
```

### CI 自动打包（推荐）

仓库内置 [.github/workflows/build.yaml](.github/workflows/build.yaml)，覆盖 Windows / macOS / Linux 三平台：

- **发布 Release**：创建 Release（或推 tag 后由 GitHub 页面创建）即自动构建，安装包自动上传到 Release 附件
- **手动触发**：Actions 页面选择「Wails build」→ Run workflow，产物在运行详情的 Artifacts 中下载

> 首次运行 `wails dev` 会执行 `npm install` 并自动生成 `frontend/wailsjs/` 前端绑定代码（连接 Go 方法用），无需手动干预。
> 若 `go mod` 依赖版本冲突，先执行 `go mod tidy`。

## 使用说明

1. 启动后点右上角「新建连接」
2. 填写连接串（如 `192.168.1.10:2181,192.168.1.11:2181`），如需 chroot 填 `/dubbo` 等，开启认证时填 `user:pass`
3. 连接成功后在左侧节点树浏览 / 操作节点：
   - 选中节点后可查看/编辑数据、查看 Stat、管理 ACL、注册 Watch
   - 「删除节点」默认删除空节点；勾选「递归删除」可删除包含子孙的节点
4. 右上角「集群监控」切换四字命令面板，查看 ZK Server 运行指标

## 常见问题

### `Could not resolve "../wailsjs/go/main/App" from "src/components/..."`

原因：Wails 生成的绑定代码在 `frontend/wailsjs/`，而 `src/components/` 下的组件用 `../wailsjs` 会解析到 `src/wailsjs/`。
解决：将组件内的导入改为 `../../wailsjs/go/main/App`，并删除误建的 `frontend/src/wailsjs/` 空目录。

### mntr / cons 等四字命令执行失败

Zookeeper 默认只放行部分四字命令。在 Server 的 `zoo.cfg` 中添加：

```
4lw.commands.whitelist=ruok,stat,srvr,mntr,cons,wchs,envi
```

### 连接报 `KeeperErrorCode = NoAuth`

该 ZK 开启了 digest 认证，请在新建连接时填写 `user:pass`。

## 后续可扩展方向

- 节点数据 Base64 / 二进制视图切换
- 按节点名 / 数据内容全文搜索
- Dubbo / Kafka 等基于 ZK 的应用元数据导航
- 节点数据差异对比（Compare）
- 四字命令定时轮询自动刷新（对标 ES-King 的实时监控）

## 免责声明

本项目仅供学习与内部运维使用。
