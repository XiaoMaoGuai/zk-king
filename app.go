package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- 数据结构 ----------

type ConnectionInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Hosts     string   `json:"hosts"`
	Chroot    string   `json:"chroot"`
	Connected bool     `json:"connected"`
	Servers   []string `json:"servers"`
}

type NodeStat struct {
	Czxid          int64 `json:"czxid"`
	Mzxid          int64 `json:"mzxid"`
	Ctime          int64 `json:"ctime"`
	Mtime          int64 `json:"mtime"`
	Version        int32 `json:"version"`
	Cversion       int32 `json:"cversion"`
	Aversion       int32 `json:"aversion"`
	EphemeralOwner int64 `json:"ephemeralOwner"`
	DataLength     int32 `json:"dataLength"`
	NumChildren    int32 `json:"numChildren"`
	Pzxid          int64 `json:"pzxid"`
}

type ACLInfo struct {
	Scheme string `json:"scheme"`
	ID     string `json:"id"`
	Perms  string `json:"perms"`
	Raw    int32  `json:"raw"`
}

type NodeDetail struct {
	Path     string    `json:"path"`
	Data     string    `json:"data"`
	Stat     NodeStat  `json:"stat"`
	ACL      []ACLInfo `json:"acl"`
	Children []string  `json:"children"`
}

type WatchEvent struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Type string `json:"type"`
	Time string `json:"time"`
}

type zkSession struct {
	info    ConnectionInfo
	conn    *zk.Conn
	mu      sync.Mutex
	watches map[string]chan struct{} // path -> cancel channel
}

type App struct {
	ctx      context.Context
	mu       sync.Mutex
	sessions map[string]*zkSession
	seq      int
}

func NewApp() *App {
	return &App{
		sessions: make(map[string]*zkSession),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func statToNodeStat(s *zk.Stat) NodeStat {
	if s == nil {
		return NodeStat{}
	}
	return NodeStat{
		Czxid:          s.Czxid,
		Mzxid:          s.Mzxid,
		Ctime:          s.Ctime,
		Mtime:          s.Mtime,
		Version:        s.Version,
		Cversion:       s.Cversion,
		Aversion:       s.Aversion,
		EphemeralOwner: s.EphemeralOwner,
		DataLength:     s.DataLength,
		NumChildren:    s.NumChildren,
		Pzxid:          s.Pzxid,
	}
}

func permsToString(p int32) string {
	var b strings.Builder
	if p&zk.PermCreate != 0 {
		b.WriteString("c")
	}
	if p&zk.PermDelete != 0 {
		b.WriteString("d")
	}
	if p&zk.PermRead != 0 {
		b.WriteString("r")
	}
	if p&zk.PermWrite != 0 {
		b.WriteString("w")
	}
	if p&zk.PermAdmin != 0 {
		b.WriteString("a")
	}
	return b.String()
}

func aclToInfos(acls []zk.ACL) []ACLInfo {
	out := make([]ACLInfo, 0, len(acls))
	for _, a := range acls {
		out = append(out, ACLInfo{Scheme: a.Scheme, ID: a.ID, Perms: permsToString(a.Perms), Raw: a.Perms})
	}
	return out
}

// parseACL 解析多行/逗号分隔的 ACL 描述，如 "world:anyone:cdrwa,digest:user:hash:cdrwa"
func parseACL(s string) ([]zk.ACL, error) {
	var out []zk.ACL
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			return nil, fmt.Errorf("ACL 格式错误: %s（应为 scheme:id:perms）", line)
		}
		scheme := parts[0]
		id := strings.Join(parts[1:len(parts)-1], ":")
		permsStr := parts[len(parts)-1]
		var perms int32
		for _, ch := range permsStr {
			switch ch {
			case 'c', 'C':
				perms |= zk.PermCreate
			case 'd', 'D':
				perms |= zk.PermDelete
			case 'r', 'R':
				perms |= zk.PermRead
			case 'w', 'W':
				perms |= zk.PermWrite
			case 'a', 'A':
				perms |= zk.PermAdmin
			default:
				return nil, fmt.Errorf("ACL 权限字符非法: %c（仅支持 c d r w a）", ch)
			}
		}
		out = append(out, zk.ACL{Perms: perms, Scheme: scheme, ID: id})
	}
	if len(out) == 0 {
		return nil, errors.New("ACL 不能为空")
	}
	return out, nil
}

// ---------- 连接管理 ----------

func (a *App) GetConnections() []ConnectionInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]ConnectionInfo, 0, len(a.sessions))
	for _, s := range a.sessions {
		connected := true
		if s.conn != nil {
			st := s.conn.State()
			connected = st != zk.StateDisconnected && st != zk.StateExpired
		}
		info := s.info
		info.Connected = connected
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (a *App) ConnectZk(name, hosts, chroot, auth string, timeoutSec int) (ConnectionInfo, error) {
	if strings.TrimSpace(hosts) == "" {
		return ConnectionInfo{}, errors.New("hosts 不能为空，例如 192.168.1.10:2181,192.168.1.11:2181")
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}

	servers := []string{}
	for _, h := range strings.Split(hosts, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			servers = append(servers, h)
		}
	}
	if chroot != "" && !strings.HasPrefix(chroot, "/") {
		chroot = "/" + chroot
	}
	connectServers := servers
	if chroot != "" && chroot != "/" {
		connectServers = make([]string, len(servers))
		for i, s := range servers {
			connectServers[i] = s + chroot
		}
	}

	conn, sessionEvents, err := zk.Connect(connectServers, time.Duration(timeoutSec)*time.Second, zk.WithLogInfo(false))
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("连接失败: %v", err)
	}
	if auth != "" {
		if err := conn.AddAuth("digest", []byte(auth)); err != nil {
			conn.Close()
			return ConnectionInfo{}, fmt.Errorf("认证失败: %v", err)
		}
	}
	// 连通性校验
	if _, _, err := conn.Children("/"); err != nil {
		conn.Close()
		return ConnectionInfo{}, fmt.Errorf("无法访问根节点: %v", err)
	}

	a.mu.Lock()
	a.seq++
	id := fmt.Sprintf("conn-%d", a.seq)
	sess := &zkSession{
		info: ConnectionInfo{
			ID: id, Name: name, Hosts: hosts, Chroot: chroot, Connected: true, Servers: servers,
		},
		conn:    conn,
		watches: make(map[string]chan struct{}),
	}
	a.sessions[id] = sess
	a.mu.Unlock()

	// 监听会话断开事件并推送给前端
	go func() {
		for ev := range sessionEvents {
			runtime.EventsEmit(a.ctx, "zk-session", map[string]interface{}{
				"id": id, "state": ev.State.String(),
			})
			if ev.State == zk.StateDisconnected || ev.State == zk.StateExpired {
				return
			}
		}
	}()

	return sess.info, nil
}

func (a *App) DisconnectZk(id string) error {
	a.mu.Lock()
	sess, ok := a.sessions[id]
	if ok {
		delete(a.sessions, id)
	}
	a.mu.Unlock()
	if !ok {
		return errors.New("连接不存在")
	}
	sess.mu.Lock()
	for _, cancel := range sess.watches {
		close(cancel)
	}
	sess.mu.Unlock()
	sess.conn.Close()
	return nil
}

func (a *App) getSession(id string) (*zkSession, error) {
	a.mu.Lock()
	sess, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok || sess.conn == nil {
		return nil, errors.New("连接不存在或已断开")
	}
	return sess, nil
}

// ---------- 节点操作 ----------

// GetChildren 返回某路径下子节点名列表（不含全路径）
func (a *App) GetChildren(id, path string) ([]string, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	children, _, err := sess.conn.Children(path)
	if err != nil {
		return nil, fmt.Errorf("读取子节点失败 %s: %v", path, err)
	}
	if children == nil {
		children = []string{}
	}
	sort.Strings(children)
	return children, nil
}

// GetNode 获取节点数据 + Stat + ACL + 子节点
func (a *App) GetNode(id, path string) (NodeDetail, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return NodeDetail{}, err
	}
	data, stat, err := sess.conn.Get(path)
	if err != nil {
		return NodeDetail{}, fmt.Errorf("读取节点失败 %s: %v", path, err)
	}
	acls, _, err := sess.conn.GetACL(path)
	if err != nil {
		return NodeDetail{}, fmt.Errorf("读取 ACL 失败 %s: %v", path, err)
	}
	children, _, err := sess.conn.Children(path)
	if err != nil {
		return NodeDetail{}, fmt.Errorf("读取子节点失败 %s: %v", path, err)
	}
	if children == nil {
		children = []string{}
	}
	sort.Strings(children)
	return NodeDetail{
		Path:     path,
		Data:     string(data),
		Stat:     statToNodeStat(stat),
		ACL:      aclToInfos(acls),
		Children: children,
	}, nil
}

// CreateNode 创建节点；nodeType: 0=持久 1=临时 2=持久顺序 4=容器
func (a *App) CreateNode(id, parentPath, nodeName, data string, nodeType int) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	if parentPath == "" {
		parentPath = "/"
	}
	path := parentPath
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	path += strings.TrimPrefix(nodeName, "/")
	if path == "/" || path == "" {
		return errors.New("节点路径不能为空")
	}
	acl := zk.WorldACL(zk.PermAll) // WorldACL 已返回 []zk.ACL
	_, err = sess.conn.Create(path, []byte(data), int32(nodeType), acl)
	if err != nil {
		return fmt.Errorf("创建节点失败: %v", err)
	}
	return nil
}

// UpdateNode 更新节点数据
func (a *App) UpdateNode(id, path, data string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	_, err = sess.conn.Set(path, []byte(data), -1)
	if err != nil {
		return fmt.Errorf("更新节点失败: %v", err)
	}
	return nil
}

// DeleteNode 删除节点（version=-1 任意版本）
func (a *App) DeleteNode(id, path string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	if path == "/" {
		return errors.New("不能删除根节点")
	}
	err = sess.conn.Delete(path, -1)
	if err != nil {
		return fmt.Errorf("删除节点失败: %v", err)
	}
	return nil
}

// DeleteNodeRecursive 递归删除节点及其所有子孙节点
func (a *App) DeleteNodeRecursive(id, path string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	if path == "/" || path == "" {
		return errors.New("不能删除根节点")
	}
	return deleteRecursive(sess.conn, strings.TrimSuffix(path, "/"))
}

func deleteRecursive(conn *zk.Conn, path string) error {
	children, _, err := conn.Children(path)
	if err != nil {
		return fmt.Errorf("读取子节点失败 %s: %v", path, err)
	}
	for _, c := range children {
		if err := deleteRecursive(conn, path+"/"+c); err != nil {
			return err
		}
	}
	if err := conn.Delete(path, -1); err != nil {
		return fmt.Errorf("删除节点失败 %s: %v", path, err)
	}
	return nil
}

// SetNodeACL 设置节点 ACL，aclText 形如 "world:anyone:cdrwa\ndigest:user:hash:cdrwa"
func (a *App) SetNodeACL(id, path, aclText string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	acls, err := parseACL(aclText)
	if err != nil {
		return err
	}
	_, err = sess.conn.SetACL(path, acls, -1)
	if err != nil {
		return fmt.Errorf("设置 ACL 失败: %v", err)
	}
	return nil
}

// ---------- 监听器 ----------

func (a *App) AddWatch(id, path string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	sess.mu.Lock()
	if _, exists := sess.watches[path]; exists {
		sess.mu.Unlock()
		return nil
	}
	cancel := make(chan struct{})
	sess.watches[path] = cancel
	sess.mu.Unlock()

	go func() {
		for {
			_, _, evCh, err := sess.conn.ChildrenW(path)
			if err != nil {
				runtime.EventsEmit(a.ctx, "zk-watch", map[string]interface{}{
					"id": id, "path": path, "type": "ERROR", "error": err.Error(),
				})
				return
			}
			select {
			case ev := <-evCh:
				runtime.EventsEmit(a.ctx, "zk-watch", map[string]interface{}{
					"id":   id,
					"path": path,
					"type": ev.Type.String(),
					"time": time.Now().Format("15:04:05"),
				})
			case <-cancel:
				return
			}
		}
	}()
	return nil
}

func (a *App) RemoveWatch(id, path string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	sess.mu.Lock()
	if cancel, ok := sess.watches[path]; ok {
		close(cancel)
		delete(sess.watches, path)
	}
	sess.mu.Unlock()
	return nil
}

func (a *App) GetWatches(id string) ([]string, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	out := make([]string, 0, len(sess.watches))
	for p := range sess.watches {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// ---------- 四字命令监控 ----------

// FourLetterWord 发送 ZK 四字命令到指定 server（ruok/stat/srvr/mntr/cons/wchs 等）
func (a *App) FourLetterWord(id, host, cmd string) (string, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return "", err
	}
	if host == "" {
		if len(sess.info.Servers) > 0 {
			host = sess.info.Servers[0]
		} else {
			return "", errors.New("请选择一个 server")
		}
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", errors.New("命令不能为空")
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("连接 %s 失败: %v", host, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", err
	}
	buf, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
