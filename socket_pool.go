package play

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type GroupSocket struct {
	mu          sync.Mutex
	groups      map[string]*socketWeightPool
	maxIdle     int           // 连接池中每台服务器最大空闲连接数
	maxConn     int           // 连接池中每台服务器最多连接数 0：表示不限制
	maxWaitTime time.Duration //获取连接最大等待时间 0：表示不限制
	hosts       map[string]map[string]int
}

type socketWeightPool struct {
	mu            sync.Mutex
	hostsWeighted map[string]*weighted
}

type weighted struct {
	host                  string
	weight, currentWeight int
	connChans             chan *SocketConn
}

type SocketConn struct {
	net.Conn
	w    *weighted
	dead bool
}

func newWeightPool(hosts map[string]int, maxIdle int) *socketWeightPool {
	weightPool := &socketWeightPool{hostsWeighted: make(map[string]*weighted, len(hosts))}
	for k, v := range hosts {
		weightPool.hostsWeighted[k] = &weighted{
			host:      k,
			weight:    v,
			connChans: make(chan *SocketConn, maxIdle),
		}
	}

	return weightPool
}

func NewGroupSocket(maxIdle int) *GroupSocket {
	return &GroupSocket{groups: make(map[string]*socketWeightPool, 1), maxIdle: maxIdle, hosts: make(map[string]map[string]int, 1)}
}

func (gs *GroupSocket) SetGroup(groupName string, hosts map[string]int) {
	gs.mu.Lock()
	gs.mu.Unlock()

	if _, ok := gs.hosts[groupName]; ok {
		for _, v := range gs.groups[groupName].hostsWeighted {
			v.close()
		}
	}

	gs.groups[groupName] = newWeightPool(hosts, gs.maxIdle)
	gs.hosts[groupName] = hosts
}

func (gs *GroupSocket) SetHost(groupName, host string, weight int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if ghost, ok := gs.hosts[groupName]; !ok {
		gs.hosts[groupName] = map[string]int{host: weight}
		gs.groups[groupName] = newWeightPool(map[string]int{host: weight}, gs.maxIdle)
	} else {
		if _, ok := ghost[host]; !ok {
			gs.groups[groupName].hostsWeighted[host] = &weighted{host: host, weight: weight, connChans: make(chan *SocketConn, gs.maxIdle)}
		} else {
			gs.groups[groupName].hostsWeighted[host].weight = weight
		}

		ghost[host] = weight
	}
}

func (gs *GroupSocket) Delete(groupName, host string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if ghost, ok := gs.hosts[groupName]; ok {
		pools := gs.groups[groupName]
		if host == "" {
			delete(gs.hosts, groupName)
			delete(gs.groups, groupName)

			for _, v := range pools.hostsWeighted {
				v.close()
			}
		} else {
			if _, ok := ghost[host]; ok {
				w := pools.hostsWeighted[host]
				delete(pools.hostsWeighted, host)
				delete(ghost, host)
				w.close()
				if len(ghost) == 0 {
					delete(gs.hosts, groupName)
					delete(gs.groups, groupName)
				}
			}
		}
	}
}

func (gs *GroupSocket) GetHosts() map[string]map[string]int {
	return gs.hosts
}

func (gs *GroupSocket) GetSocketConnByGroupName(groupName string) (*SocketConn, error) {
	var ok bool
	var pool *socketWeightPool
	if pool, ok = gs.groups[groupName]; !ok {
		gs.mu.Lock()
		if pool, ok = gs.groups[groupName]; !ok {
			gs.groups[groupName] = newWeightPool(gs.hosts[groupName], gs.maxIdle)
			pool = gs.groups[groupName]
		}
		gs.mu.Unlock()
	}
	return pool.getWeightConn()
}

func (p *socketWeightPool) getWeightConn() (*SocketConn, error) {
	if weightedHost, err := p.next(); err != nil {
		return nil, err
	} else {
		return weightedHost.getConn()
	}
}

func (p *socketWeightPool) next() (*weighted, error) {
	var total int
	var best *weighted
	var err error

	p.mu.Lock()
	for _, v := range p.hostsWeighted {
		total += v.weight
		v.currentWeight += v.weight
		if best == nil || v.currentWeight > best.currentWeight {
			best = v
		}
	}
	if best != nil {
		best.currentWeight -= total
	} else {
		err = errors.New("weight pool is empty")
	}
	p.mu.Unlock()

	return best, err
}

func (w *weighted) getConn() (*SocketConn, error) {
	select {
	case conn := <-w.connChans:
		return conn, nil
	default:
		nconn, err := net.DialTimeout("tcp", w.host, 50*time.Millisecond)
		if err != nil {
			return nil, err
		}
		fmt.Println("new connect", w.host)
		return &SocketConn{Conn: nconn, w: w}, nil
	}
}

func (w *weighted) putConn(conn *SocketConn) error {
	if conn == nil || conn.Conn == nil {
		return errors.New("connection is nil")
	}

	if w.connChans == nil {
		conn.Conn.Close()
	}

	select {
	case w.connChans <- conn:
		return nil
	default:
		return conn.Conn.Close()
	}
}

func (w *weighted) close() {
	chans := w.connChans
	w.connChans = nil

	go func() {
		for conn := range chans {
			if conn != nil {
				conn.dead = true
				conn.Conn.Close()
			}
		}
	}()
}

func (conn *SocketConn) SetDead() {
	conn.dead = true
}

func (conn *SocketConn) Close() error {
	if conn.dead {
		if conn.Conn != nil {
			fmt.Println(conn.Conn.Close())
			return nil
		}
		return nil
	}
	return conn.w.putConn(conn)
}
