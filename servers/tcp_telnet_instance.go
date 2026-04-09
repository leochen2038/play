package servers

import (
	"context"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
	"io"
	"log"
	"net"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

type TcpTelnetInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	actions     map[string]*play.ActionUnit
	sortedNames []string
	packer      play.IPacker
}

func NewTcpTelnetInstance(name string, addr string, hook play.IServerHook, packer play.IPacker, defaultActionTimeout time.Duration) *TcpTelnetInstance {
	if packer == nil {
		packer = packers.NewTelnetPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	return &TcpTelnetInstance{info: play.NewInstanceInfo(name, addr, play.SERVER_TYPE_TCP, defaultActionTimeout), packer: packer, hook: hook, ctrl: new(play.InstanceCtrl), actions: make(map[string]*play.ActionUnit)}
}

// 空闲管理器定义
type IdleManager struct {
	lastActive    time.Time
	maxIdleTime   time.Duration
	warnBefore    time.Duration
	warnChan      chan struct{}
	timeoutChan   chan struct{}
	activityChan  chan struct{}
	shutdownChan  chan struct{}
	extensionChan chan time.Duration
	idleDuration  time.Duration
	warningCount  int
	mutex         sync.Mutex
}

// 创建新的空闲管理器
func NewIdleManager(maxIdle, warnBefore time.Duration) *IdleManager {
	mgr := &IdleManager{
		lastActive:    time.Now(),
		maxIdleTime:   maxIdle,
		warnBefore:    warnBefore,
		activityChan:  make(chan struct{}, 1),
		shutdownChan:  make(chan struct{}),
		warnChan:      make(chan struct{}, 1),
		timeoutChan:   make(chan struct{}, 1),
		extensionChan: make(chan time.Duration, 1),
	}
	go mgr.monitor()
	return mgr
}

// 延长超时时间
func (m *IdleManager) Extend(duration time.Duration) {
	m.mutex.Lock()
	m.maxIdleTime += duration
	m.warningCount = 0 // 重置警告计数
	m.mutex.Unlock()
	select {
	case m.extensionChan <- duration:
	default:
	}
}

// 停止监控
func (m *IdleManager) Shutdown() {
	close(m.shutdownChan)
}

// 获取剩余时间
func (m *IdleManager) Remaining() time.Duration {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	elapsed := time.Since(m.lastActive)
	remaining := m.maxIdleTime - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// 获取空闲持续时间
func (m *IdleManager) IdleDuration() time.Duration {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return time.Since(m.lastActive)
}

// 监控协程
func (m *IdleManager) monitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastWarn time.Time
	minWarnInterval := 15 * time.Second

	for {
		select {
		case <-m.shutdownChan:
			return

		case <-m.activityChan:
			// 重置活动时间
			lastWarn = time.Time{}
			m.mutex.Lock()
			m.warningCount = 0
			m.mutex.Unlock()

		case <-ticker.C:
			m.mutex.Lock()
			idleDuration := time.Since(m.lastActive)
			remaining := m.maxIdleTime - idleDuration
			m.idleDuration = idleDuration
			m.mutex.Unlock()

			// 处理延长
			select {
			case dur := <-m.extensionChan:
				m.mutex.Lock()
				m.maxIdleTime += dur
				remaining = m.maxIdleTime - idleDuration
				m.mutex.Unlock()
			default:
			}

			// 提前警告（限制频率）
			if remaining <= m.warnBefore && remaining > 0 {
				if time.Since(lastWarn) > minWarnInterval {
					select {
					case m.warnChan <- struct{}{}:
						lastWarn = time.Now()
						m.mutex.Lock()
						m.warningCount++
						m.mutex.Unlock()
					default:
					}
				}
			}

			// 完整超时
			if idleDuration >= m.maxIdleTime {
				select {
				case m.timeoutChan <- struct{}{}:
				default:
				}
				return
			}
		}
	}
}

// 获取超时配置
func getTimeoutConfig(s *play.Session) time.Duration {
	// 管理员用户更长超时
	if isAdminUser(s) {
		return 30 * time.Minute
	}
	return 5 * time.Minute
}

// 发送心跳
func sendHeartbeats(s *play.Session, mgr *IdleManager) {
	const (
		IAC = 0xFF
		NOP = 0xF1
	)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 仅空闲时发送
			if mgr.IdleDuration() > 1*time.Minute {
				// 发送不可见心跳（防止干扰用户输入）
				s.Conn.Tcp.Conn.Write([]byte{IAC, NOP})
				//s.Conn.Tcp.Conn.Write([]byte{0xAA, 0x55, 0x01})
			}
		case <-s.Context().Done():
			return
		case <-mgr.shutdownChan:
			return
		}
	}
}

// 检测有效活动
func isRealActivity(data []byte) bool {
	for _, b := range data {
		// 排除控制字符和空格
		if b > 32 && b != 127 { // 127 = DEL
			return true
		}
	}
	return false
}

// 处理延长命令
func handleExtendCommand(input string, mgr *IdleManager, conn net.Conn) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(lower, "extend") {
		mgr.Extend(5 * time.Minute)
		conn.Write([]byte("Connection extended by 5 minutes\r\n"))
		return true
	}
	return false
}

// 主连接处理函数
func (i *TcpTelnetInstance) onReady(s *play.Session) (err error) {
	const readBufferSize = 4096
	var n int
	defer func() {
		if r := recover(); r != nil {
			log.Printf("onReady panic: %v", r)
			err = fmt.Errorf("runtime error: %v", r)
		}
	}()

	conn := s.Conn.Tcp.Conn
	buffer := make([]byte, readBufferSize)

	// 获取超时配置
	maxIdleTime := getTimeoutConfig(s)
	warnBefore := time.Duration(float64(maxIdleTime) * 0.2) // 提前20%警告

	// 创建空闲管理器
	idleMgr := NewIdleManager(maxIdleTime, warnBefore)
	defer idleMgr.Shutdown()

	// 启动心跳
	go sendHeartbeats(s, idleMgr)

	// 发送欢迎消息
	welcome := fmt.Sprintf("Connected to Telnet service | Session timeout: %d minutes\r\n",
		int(maxIdleTime.Minutes()))
	conn.Write([]byte(welcome))

	for {
		select {
		case <-s.Context().Done():
			return s.Context().Err()

		case <-idleMgr.warnChan:
			// 发送多语言警告
			seconds := int(idleMgr.Remaining().Seconds())
			msg := fmt.Sprintf("WARNING: Connection will timeout in %d seconds", seconds)

			// 添加ANSI颜色
			color := "\x1b[33m" // 黄色
			if seconds < 30 {
				color = "\x1b[31m" // 红色
			}

			conn.Write([]byte(fmt.Sprintf(
				"\r\n%s%s\x1b[0m\r\n",
				color, msg,
			)))

			// 提供续期选项（最多提示3次）
			idleMgr.mutex.Lock()
			warnCount := idleMgr.warningCount
			idleMgr.mutex.Unlock()

			if warnCount <= 3 {
				extendMsg := "Type 'extend' to add 5 minutes"
				conn.Write([]byte(fmt.Sprintf("\x1b[36m%s\x1b[0m\r\n", extendMsg)))
			}

		case <-idleMgr.timeoutChan:
			// 记录超时日志
			log.Printf("Connection timeout: %s | Duration: %s",
				conn.RemoteAddr(), idleMgr.idleDuration)

			// 发送超时通知
			timeoutMsg := "ERROR: Connection idle timeout"
			conn.Write([]byte(fmt.Sprintf("\r\n\x1b[31m%s\x1b[0m\r\n", timeoutMsg)))
			return nil

		default:
			// 设置带超时的读取
			timeout := min(10*time.Second, idleMgr.Remaining())
			conn.SetReadDeadline(time.Now().Add(timeout))

			n, err = conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if err == io.EOF {
					log.Printf("Client disconnected: %s", conn.RemoteAddr())
					return nil
				}
				return err
			}

			// 检测有效活动
			if isRealActivity(buffer[:n]) {
				idleMgr.Activity()
			}

			// 处理延长请求
			if handleExtendCommand(string(buffer[:n]), idleMgr, conn) {
				continue
			}

			// 处理常规数据
			s.Conn.Tcp.Surplus = append(s.Conn.Tcp.Surplus, buffer[:n]...)
			if err = i.processBuffer(s); err != nil {
				return err
			}
		}
	}
}

// 处理缓冲区数据
func (i *TcpTelnetInstance) processBuffer(s *play.Session) error {
	processed := 0
	maxCommands := 100
	conn := s.Conn.Tcp

	for processed < maxCommands && len(conn.Surplus) > 0 {
		req, unpackErr := i.packer.Unpack(s.Conn)
		if unpackErr != nil {
			s.Write(&play.Response{Error: unpackErr})
			processed++
			conn.Surplus = nil
			continue
		}

		if req == nil {
			break // 没有完整命令
		}

		// 更新连接版本
		if req.Version > conn.Version {
			conn.Version = req.Version
		}

		// 处理请求
		if err := play.DoRequest(s.Context(), s, req); err != nil {
			return err
		}

		processed++
	}

	return nil
}

// 记录活动
func (m *IdleManager) Activity() {
	m.mutex.Lock()
	m.lastActive = time.Now()
	m.mutex.Unlock()
	select {
	case m.activityChan <- struct{}{}:
	default:
	}
}

// 辅助函数
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// 检查管理员用户
func isAdminUser(s *play.Session) bool {
	// 实现根据会话认证信息判断
	return true
}

func (i *TcpTelnetInstance) Info() play.IInstanceInfo {
	return i.info
}

func (i *TcpTelnetInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *TcpTelnetInstance) Packer() play.IPacker {
	return i.packer
}

func (i *TcpTelnetInstance) Transport(conn *play.Conn, data []byte) error {
	_, err := conn.Tcp.Conn.Write(data)
	return err
}

func (i *TcpTelnetInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *TcpTelnetInstance) Run(listener net.Listener, udplistener net.PacketConn) error {
	for {
		var err error
		var conn net.Conn
		if conn, err = listener.Accept(); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return err
			}
		}

		go func(err error, conn net.Conn) {
			s := play.NewSession(context.Background(), i)
			s.Conn.Tcp.Conn = conn

			defer func() {
				if panicInfo := recover(); panicInfo != nil {
					fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
				}
				if s.Conn.Tcp.Conn != nil {
					_ = s.Conn.Tcp.Conn.Close()
				}
			}()

			defer func() {
				i.hook.OnClose(s, err)
			}()
			i.hook.OnConnect(s, err)

			if err == nil {
				err = i.onReady(s)
			}
		}(err, conn)
	}
}

func (i *TcpTelnetInstance) Close() {
	i.ctrl.WaitTask()
}

func (i *TcpTelnetInstance) Network() string {
	return "tcp"
}

func (i *TcpTelnetInstance) ActionUnitNames() []string {
	return append([]string(nil), i.sortedNames...)
}

func (i *TcpTelnetInstance) LookupActionUnit(requestName string) *play.ActionUnit {
	return i.actions[requestName]
}

func (i *TcpTelnetInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	return bindActionSpace(i, spaceName, actionPackages)
}

func (i *TcpTelnetInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

func (i *TcpTelnetInstance) AddActionUnits(units ...*play.ActionUnit) error {
	for _, u := range units {
		if i.actions[u.RequestName] != nil {
			return errors.New("action unit " + u.RequestName + " is already exists in " + i.info.Name())
		}
		i.actions[u.RequestName] = u
		i.sortedNames = append(i.sortedNames, u.RequestName)
	}
	sort.Strings(i.sortedNames)
	return nil
}
