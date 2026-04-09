package servers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"runtime/debug"
	"sort"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/leochen2038/play"
	"github.com/leochen2038/play/packers"
)

type quicInstance struct {
	info        play.IInstanceInfo
	hook        play.IServerHook
	ctrl        *play.InstanceCtrl
	packer      play.IPacker
	quicConfig  *quic.Config
	tlsconfig   *tls.Config
	onceStream  bool
	isClose     bool
	actions     map[string]*play.ActionUnit
	sortedNames []string
	quicServer  *quic.Listener
}

func NewQuicInstance(name string, addr string, hook play.IServerHook, packer play.IPacker, defaultActionTimeout time.Duration) *quicInstance {
	if packer == nil {
		packer = packers.NewPlayPacker()
	}
	if hook == nil {
		hook = defaultHook{}
	}
	if defaultActionTimeout == 0 {
		defaultActionTimeout = defaultTimeout
	}
	return &quicInstance{onceStream: true, info: play.NewInstanceInfo(name, addr, play.SERVER_TYPE_QUIC, defaultActionTimeout), packer: packer, hook: hook, ctrl: new(play.InstanceCtrl), actions: make(map[string]*play.ActionUnit)}
}

func (i *quicInstance) SetTlsConfig(tlsconfig *tls.Config) {
	i.tlsconfig = tlsconfig
}

func (i *quicInstance) SetQuicConfig(config *quic.Config) {
	i.quicConfig = config
}

func (i *quicInstance) Info() play.IInstanceInfo {
	return i.info
}

func (i *quicInstance) Ctrl() *play.InstanceCtrl {
	return i.ctrl
}

func (i *quicInstance) Hook() play.IServerHook {
	return i.hook
}

func (i *quicInstance) Packer() play.IPacker {
	return i.packer
}

func (i *quicInstance) Transport(conn *play.Conn, data []byte) (err error) {
	var stream quic.Stream
	if conn.Quic.Stream != nil {
		stream = conn.Quic.Stream
	} else {
		if stream, err = conn.Quic.Conn.OpenStreamSync(context.Background()); err != nil {
			return
		}
	}

	_, err = stream.Write(data)
	return err
}

func (i *quicInstance) Network() string {
	return "udp"
}

func (i *quicInstance) Run(listener net.Listener, udplistener net.PacketConn) (err error) {
	var tlsconfig *tls.Config
	if i.tlsconfig == nil {
		tlsconfig = generateTLSConfig([]string{i.info.Name()})
	}
	i.quicServer, err = quic.Listen(udplistener, tlsconfig, i.quicConfig)

	if err != nil {
		return err
	}

	for {
		conn, err := i.quicServer.Accept(context.Background())
		if err != nil {
			return err
		}
		if i.isClose {
			conn.CloseWithError(0, "server is closed")
			continue
		}

		// 启用新协程处理新stream
		go func(conn quic.Connection) {
			s := play.NewSession(context.Background(), i)
			s.Conn.Quic.Conn = conn
			defer func() {
				if panicInfo := recover(); panicInfo != nil {
					fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
				}
			}()
			defer func() {
				i.hook.OnClose(s, err)
			}()
			i.hook.OnConnect(s, err)

			for {
				select {
				case <-s.Context().Done():
					fmt.Println("session is closed by " + s.Context().Err().Error())
					return
				default:
					stream, err := conn.AcceptStream(context.Background())
					if err != nil {
						return
					}
					if i.isClose {
						stream.Close()
						return
					}
					go func(strean quic.Stream) {
						ss := play.NewSession(s.Context(), i)
						ss.Conn.Quic.Conn = conn
						ss.Conn.Quic.Stream = stream

						defer func() {
							if panicInfo := recover(); panicInfo != nil {
								fmt.Printf("panic: %v\n%v", panicInfo, string(debug.Stack()))
							}
							stream.Close()
						}()
						if err == nil {
							if i.onceStream {
								err = i.onReadyOnce(ss)
							} else {
								err = i.onReady(ss)
							}
						}
					}(stream)
				}
			}
		}(conn)
	}
}

func (i *quicInstance) onReadyOnce(s *play.Session) (err error) {
	var request *play.Request
	if request, err = i.packer.Unpack(s.Conn); err != nil {
		return
	}

	s.Conn.Quic.Stream.CancelRead(0)
	s.Conn.Quic.Version = request.Version
	if err = play.DoRequest(context.Background(), s, request); err != nil {
		return err
	}
	s.Conn.Quic.Stream = nil
	return
}

func (i *quicInstance) onReady(s *play.Session) (err error) {
	var request *play.Request

	for {
		if i.isClose {
			s.Conn.Quic.Stream.CancelRead(0)
		}
		if request, err = i.packer.Unpack(s.Conn); err != nil {
			return
		}
		if request == nil {
			continue
		}
		if request.Version > s.Conn.Quic.Version {
			s.Conn.Quic.Version = request.Version
		}
		if err = play.DoRequest(context.Background(), s, request); err != nil {
			return
		}
	}
}

func (i *quicInstance) Close() {
	i.isClose = true
	i.ctrl.WaitTask()
	time.Sleep(1 * time.Second)
}

func (i *quicInstance) SetOnceStream(onceStream bool) {
	i.onceStream = onceStream
}

func generateTLSConfig(protos []string) *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   protos,
	}
}

func (i *quicInstance) ActionUnitNames() []string {
	return append([]string(nil), i.sortedNames...)
}

func (i *quicInstance) LookupActionUnit(requestName string) *play.ActionUnit {
	return i.actions[requestName]
}

func (i *quicInstance) BindActionSpace(spaceName string, actionPackages ...string) error {
	return bindActionSpace(i, spaceName, actionPackages)
}

func (i *quicInstance) UpdateActionTimeout(spaceName string, actionName string, timeout time.Duration) {
	if spaceName != "" {
		spaceName = spaceName + "."
	}
	if act := i.actions[spaceName+actionName]; act != nil {
		act.Timeout = timeout
	}
}

func (i *quicInstance) AddActionUnits(units ...*play.ActionUnit) error {
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
