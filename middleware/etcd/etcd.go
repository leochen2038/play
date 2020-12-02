package etcd

import (
	"context"
	"errors"
	"github.com/coreos/etcd/clientv3"
	"log"
	"time"
)

type EtcdAgent struct {
	client    *clientv3.Client
	Endpoints []string
}

func NewEtcdAgent(endpoints []string) (*EtcdAgent, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          100 * time.Millisecond,
		DialKeepAliveTimeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	return &EtcdAgent{client: client, Endpoints: endpoints}, nil
}

func (e EtcdAgent) Put(key string, val []byte) (err error) {
	ctx, cancelFunc := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	_, err = e.client.Put(ctx, key, string(val))
	if cancelFunc(); err != nil {
		return
	}
	return
}

func (e EtcdAgent) Del(key string) (err error) {
	ctx, cancelFunc := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	_, err = e.client.Delete(ctx, key)
	if cancelFunc(); err != nil {
		return
	}
	return
}

func (e EtcdAgent) GetEtcdValue(key string) (data []byte, err error) {
	if key == "" {
		return nil, errors.New("empty etcd key")
	}

	ctx, cancelFunc := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	resp, err := e.client.Get(ctx, key)
	if cancelFunc(); err != nil {
		return
	}

	for _, kv := range resp.Kvs {
		if string(kv.Key) == key {
			return kv.Value, nil
		}
	}

	return nil, errors.New("unable get " + key)
}

func (e EtcdAgent) GetEtcdValueWithPrefix(prefix string) (data map[string][]byte, err error) {
	if prefix == "" {
		return nil, errors.New("empty etcd prefix key")
	}

	ctx, cancelFunc := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())

	if cancelFunc(); err != nil {
		return
	}

	if resp.Count > 0 {
		data = make(map[string][]byte, resp.Count)
		for _, kv := range resp.Kvs {
			key := string(kv.Key)
			data[key] = kv.Value
		}
	}

	return
}

func (e EtcdAgent) StartKeepAlive(key string, ttl int64, change func() (string, bool, error)) {
	if key == "" {
		return
	}
	go func() {
		var err error
		defer func() {
			if panicInfo := recover(); panicInfo != nil || err != nil {
				log.Println("[playregister]", panicInfo, err)
			}
			time.Sleep(5 * time.Second)
			go e.StartKeepAlive(key, ttl, change)
		}()
		err = e.keepAlive(key, ttl, change)
	}()
}

func (e EtcdAgent) keepAlive(key string, ttl int64, change func() (string, bool, error)) (err error) {
	var leaseResp *clientv3.LeaseGrantResponse
	var aliveChan <-chan *clientv3.LeaseKeepAliveResponse

	ctx, cancelFunc := context.WithTimeout(context.TODO(), 1*time.Second)
	leaseResp, err = e.client.Grant(ctx, ttl)
	cancelFunc()
	if err != nil {
		return
	}

	ctx, cancelFunc = context.WithCancel(context.TODO())
	aliveChan, err = e.client.KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		cancelFunc()
		return
	}

	if val, isChange, err := change(); err == nil && isChange {
		ctx, cancelFunc = context.WithTimeout(context.TODO(), 1*time.Second)
		_, err = e.client.Put(ctx, key, val, clientv3.WithLease(leaseResp.ID))
		cancelFunc()
		if err != nil {
			return err
		}
	}

	for {
		select {
		case aliveResp := <-aliveChan:
			if aliveResp == nil {
				err = errors.New("etcd close")
				return
			} else {
				if val, isChange, err := change(); err == nil && isChange {
					ctx, cancelFunc = context.WithTimeout(context.TODO(), 1*time.Second)
					_, err = e.client.Put(ctx, key, val, clientv3.WithLease(leaseResp.ID))
					cancelFunc()
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return
}

func (e EtcdAgent) StartWatchChange(key string, changeNotify func(data []byte) error) {
	go func() {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				log.Println("[etcd agent panic]", panicInfo)
			}
			time.Sleep(3 * time.Second)
			go e.StartWatchChange(key, changeNotify)
		}()
		e.watchChange(key, changeNotify)
	}()
}

func (e EtcdAgent) watchChange(key string, changeNotify func(data []byte) error) {
	ctx, _ := context.WithCancel(context.TODO())
	watchChan := e.client.Watch(ctx, key)

	for {
		select {
		case watchResp := <-watchChan:
			for _, event := range watchResp.Events {
				if event.Type == clientv3.EventTypePut {
					if err := changeNotify(event.Kv.Value); err != nil {
						log.Println("[etcd agent notify failure]", err)
					}
				}
			}
		}
	}
}

func (e EtcdAgent) StartWatchChangeWithPrefix(prefix string, changeNotify func(event string, key string, data []byte) error) {
	go func() {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				log.Println("[etcd agent panic]", panicInfo)
			}
			time.Sleep(3 * time.Second)
			go e.StartWatchChangeWithPrefix(prefix, changeNotify)
		}()
		e.watchChangeWithPrefix(prefix, changeNotify)
	}()
}

func (e EtcdAgent) watchChangeWithPrefix(prefix string, changeNotify func(event string, key string, data []byte) error) {
	ctx, _ := context.WithCancel(context.TODO())
	watchChan := e.client.Watch(ctx, prefix, clientv3.WithPrefix())

	for {
		select {
		case watchResp := <-watchChan:
			for _, event := range watchResp.Events {
				var ev string
				if event.Type == clientv3.EventTypePut {
					ev = "put"
				} else if event.Type == clientv3.EventTypeDelete {
					ev = "del"
				}
				if ev != "" {
					if err := changeNotify(ev, string(event.Kv.Key), event.Kv.Value); err != nil {
						log.Println("[etcd agent noitfyWithPrefix failure]", err)
					}
				}
			}
		}
	}
}
