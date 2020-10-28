package playregister

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/config"
	"go.etcd.io/etcd/clientv3"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	buildId       = ""
	intranetIp    = ""
	exePath       = ""
	lastConfigVer = ""
	socketListen  = ""
	httpListen    = ""
)

func SetBuildId(id string) {
	buildId = id
}

func EtcdWithUrl(configUrl string) (err error) {
	var configKey string
	var runningKey string
	var endpoints []string

	if configKey, runningKey, endpoints, err = getEtcdKeyAndEndpoints(configUrl); err == nil {
		return EtcdWithArgs(configKey, runningKey, endpoints)
	}

	return
}

func EtcdWithArgs(configKey, runningKey string, endpoints []string) (err error) {
	// step 1. 获取配置信息
	var configParser play.Parser
	if configParser, err = NewEtcdParser(endpoints, configKey); err != nil {
		return
	}
	config.InitConfig(configParser)

	// step 2. 注册运行时状态
	intranetIp, _ = GetIntranetIp()
	exePath, _ = os.Executable()
	socketListen, _ = config.String("socketListen")
	httpListen, _ = config.String("httpListen")
	lastConfigVer, _ = config.String("version")

	go etcdKeepAlive(endpoints, runningKey, 3, func() string {
		if version, err := config.String("version"); err == nil && version != lastConfigVer {
			lastConfigVer = version
			return fmt.Sprintf(`{"configVer":"%s", "buildId":"%s", "ip":"%s", "pid":%d, "path":"%s", "socketListen":"%s", "httpListen":"%s"}`,
				lastConfigVer, buildId, intranetIp, os.Getpid(), exePath, socketListen, httpListen)
		}
		return ""
	})
	return
}

func etcdKeepAlive(endpoints []string, runningKey string, ttl int64, getLastVal func() string) (err error) {
	defer func() {
		if panicInfo := recover(); panicInfo != nil || err != nil {
			log.Println("[playregister]", panicInfo, err)
		}
		time.Sleep(5 * time.Second)
		go etcdKeepAlive(endpoints, runningKey, ttl, getLastVal)
	}()

	var etcdClient *clientv3.Client
	var leaseResp *clientv3.LeaseGrantResponse
	var aliveChan <-chan *clientv3.LeaseKeepAliveResponse

	if etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          100 * time.Millisecond,
		DialKeepAliveTimeout: 1 * time.Second},
	); err != nil {
		return
	}

	ctx, cancelFunc := context.WithTimeout(context.TODO(), 1*time.Second)
	leaseResp, err = etcdClient.Grant(ctx, ttl)
	cancelFunc()
	if err != nil {
		return
	}

	ctx, cancelFunc = context.WithCancel(context.TODO())
	if aliveChan, err = etcdClient.KeepAlive(ctx, leaseResp.ID); err != nil {
		cancelFunc()
		return
	}

	ctx, cancelFunc = context.WithTimeout(context.TODO(), 1*time.Second)
	_, _ = etcdClient.Put(ctx, runningKey, getLastVal(), clientv3.WithLease(leaseResp.ID))
	cancelFunc()

	for {
		select {
		case aliveResp := <-aliveChan:
			if aliveResp == nil {
				err = errors.New("etcd close")
				return
			} else {
				if newVal := getLastVal(); newVal != "" {
					ctx, cancelFunc = context.WithTimeout(context.TODO(), 1*time.Second)
					_, _ = etcdClient.Put(ctx, runningKey, newVal, clientv3.WithLease(leaseResp.ID))
					cancelFunc()
				}
			}
		}
	}
}

func getEtcdKeyAndEndpoints(configUrl string) (configKey, runningKey string, endpoints []string, err error) {
	var ip string
	var path string
	var resp *http.Response
	var responseByte []byte
	var responseMap map[string]interface{}

	if ip, err = GetIntranetIp(); err != nil {
		return
	}
	if path, err = os.Executable(); err != nil {
		return
	}

	if resp, err = http.PostForm(configUrl, url.Values{"ip": []string{ip}, "path": []string{path}}); err != nil {
		return
	}

	if responseByte, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	if err = json.Unmarshal(responseByte, &responseMap); err != nil {
		return
	}

	configKey = responseMap["configKey"].(string)
	runningKey = responseMap["serviceKey"].(string)
	for _, v := range responseMap["endpoints"].([]interface{}) {
		endpoints = append(endpoints, v.(string))
	}
	return
}

func GetIntranetIp() (ip string, err error) {
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	for _, value := range addr {
		if inet, ok := value.(*net.IPNet); ok && !inet.IP.IsLoopback() {
			if inet.IP.To4() != nil && strings.HasPrefix(inet.IP.String(), "192.168") {
				ip = inet.IP.String()
			}
		}
	}

	if ip == "" {
		err = errors.New("get ip error")
	}
	return
}
