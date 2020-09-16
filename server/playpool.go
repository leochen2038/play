package server

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	ErrClosed = errors.New("pool is closed")
)

var mu sync.RWMutex
var list = make(map[string]*SocketPool, 16)

func GetSocketPoolBy(address string) (pool *SocketPool) {
	var ok bool

	mu.Lock()
	defer mu.Unlock()

	if pool, ok = list[address]; !ok {
		pool = newPlaySocketPool(16, func() (net.Conn, error) {
			return net.DialTimeout("tcp", address, 500*time.Millisecond)
		})
		list[address] = pool
	}
	return pool
}

type SocketPool struct {
	connChans chan *PlayConn
	factory   func() (net.Conn, error)
}

func newPlaySocketPool(maxCap int, factory func() (net.Conn, error)) *SocketPool {
	pool := &SocketPool{connChans: make(chan *PlayConn, maxCap), factory: factory}
	return pool
}

func (pool *SocketPool) GetConn() (*PlayConn, error) {
	if pool.connChans == nil {
		return nil, ErrClosed
	}

	select {
	case conn := <-pool.connChans:
		if conn == nil {
			return nil, ErrClosed
		}
		return conn, nil
	default:
		nconn, err := pool.factory()
		if err != nil {
			return nil, err
		}
		return &PlayConn{Conn: nconn, pool: pool}, nil
	}
}

func (pool *SocketPool) putConn(conn *PlayConn) error {
	if conn == nil || conn.Conn == nil {
		return errors.New("connection is nil")
	}

	if pool.connChans == nil {
		conn.Close()
	}

	select {
	case pool.connChans <- conn:
		return nil
	default:
		return conn.Close()
	}

	return nil
}

func (pool *SocketPool) Close() error {
	chans := pool.connChans
	pool.connChans = nil
	for conn := range chans {
		if conn != nil {
			conn.Conn.Close()
		}
	}
	return nil
}

type PlayConn struct {
	net.Conn
	pool    *SocketPool
	Unsable bool
}

func (conn *PlayConn) Close() error {
	if conn.Unsable {
		if conn.Conn != nil {
			return conn.Conn.Close()
		}
		return nil
	}

	return conn.pool.putConn(conn)
}
