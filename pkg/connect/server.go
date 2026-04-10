package connect

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

// Server implements a NAOS Connect server.
type Server struct {
	upgrader *WebSocketUpgrader
	logger   func(string, ...any)
	token    string
	mu       sync.RWMutex
	seq      uint64
	devices  map[string]*connectedDevice
}

type connectedDevice struct {
	id         string
	deviceID   string
	deviceName string
	appName    string
	appVersion string
	connected  time.Time
	channel    *msg.Channel
}

// NewServer returns a new NAOS Connect server.
func NewServer() *Server {
	return &Server{
		upgrader: NewWebSocketUpgrader(nil),
		devices:  make(map[string]*connectedDevice),
	}
}

// SetLogger sets the server logger.
func (s *Server) SetLogger(logger func(string, ...any)) {
	s.logger = logger
}

// SetToken sets the token required to access the server. If empty, access is unrestricted.
func (s *Server) SetToken(token string) {
	s.token = token
}

// ServeHTTP serves the NAOS Connect HTTP and websocket endpoints.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		s.handleList(w, r)
	case r.URL.Path == "/connect":
		s.handleConnect(w, r)
	case strings.HasPrefix(r.URL.Path, "/device/"):
		s.handleDevice(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	if r.URL.Query().Get("token") == s.token {
		return true
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return false
	}
	if auth == s.token {
		return true
	}

	const prefix = "Bearer "
	if strings.HasPrefix(auth, prefix) && strings.TrimSpace(auth[len(prefix):]) == s.token {
		return true
	}

	return false
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.snapshot())
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r)
	if err != nil || conn == nil {
		return
	}

	channel := msg.NewChannel(NewTransport(conn), nil, 10)
	dev := &connectedDevice{
		id:        s.nextID(),
		connected: time.Now(),
		channel:   channel,
	}
	s.logf("%s reverse connected from %s", dev.label(), conn.RemoteAddr())

	if !s.probeWithRetry(dev, 4) {
		s.logf("%s metadata probe failed after 4 attempts, closing connection", dev.label())
		channel.Close()
		return
	}

	s.store(dev)
	s.logf("%s registered", dev.label())

	<-channel.Done()
	s.logf("%s disconnected", dev.label())
	s.remove(dev)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/device/"))
	if err != nil {
		http.Error(w, "invalid device id", http.StatusBadRequest)
		return
	}
	dev := s.lookup(id)
	if dev == nil {
		http.NotFound(w, r)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r)
	if err != nil || conn == nil {
		return
	}

	remoteAddr := conn.RemoteAddr().String()
	client := NewTransport(conn)
	if err := s.bridge(dev, client, remoteAddr); err != nil && !errors.Is(err, io.EOF) {
		s.logf("%s bridge error: %v", dev.label(), err)
	}
}

func (s *Server) bridge(dev *connectedDevice, client *Transport, remoteAddr string) error {
	s.logf("%s client attached from %s", dev.label(), remoteAddr)

	deviceQueue := make(msg.Queue, 64)
	dev.channel.Subscribe(deviceQueue)

	defer func() {
		dev.channel.Unsubscribe(deviceQueue)
		client.Close()
		s.logf("%s client detached", dev.label())
	}()

	errs := make(chan error, 2)
	go func() {
		for {
			data, err := client.Read()
			if err != nil {
				errs <- err
				return
			}
			if err := dev.channel.Write(deviceQueue, data); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case <-dev.channel.Done():
				errs <- io.EOF
				return
			case data := <-deviceQueue:
				if err := client.Write(data); err != nil {
					errs <- err
					return
				}
			}
		}
	}()

	return <-errs
}

func (s *Server) probeWithRetry(dev *connectedDevice, attempts int) bool {
	backoff := 250 * time.Millisecond
	timeout := time.Second

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := s.probe(dev, timeout); err == nil {
			if attempt > 1 {
				s.logf("%s metadata probe succeeded on attempt %d with timeout %s", dev.label(), attempt, timeout)
			}
			return true
		} else {
			s.logf("%s metadata probe attempt %d/%d failed with timeout %s: %v", dev.label(), attempt, attempts, timeout, err)
		}

		if attempt == attempts {
			break
		}

		select {
		case <-dev.channel.Done():
			return false
		case <-time.After(backoff):
		}

		if backoff < 2*time.Second {
			backoff *= 2
		}
		if timeout < 8*time.Second {
			timeout *= 2
		}
	}

	return false
}

func (s *Server) probe(dev *connectedDevice, timeout time.Duration) error {
	session, err := msg.OpenSessionTimeout(dev.channel, timeout)
	if err != nil {
		return err
	}
	defer func() {
		_ = session.End(timeout)
	}()

	params, err := msg.ListParams(session, timeout)
	if err != nil {
		return err
	}

	refs := map[string]uint8{}
	for _, param := range params {
		refs[param.Name] = param.Ref
	}

	required := []string{"device-id", "device-name", "app-name", "app-version"}
	var collectRefs []uint8
	for _, name := range required {
		ref, ok := refs[name]
		if !ok {
			return fmt.Errorf("missing param: %s", name)
		}
		collectRefs = append(collectRefs, ref)
	}

	updates, err := msg.CollectParams(session, collectRefs, 0, timeout)
	if err != nil {
		return err
	}

	values := map[uint8][]byte{}
	for _, update := range updates {
		values[update.Ref] = update.Value
	}

	if value := values[refs["device-id"]]; len(value) > 0 {
		dev.deviceID = string(value)
		dev.id = dev.deviceID
	} else {
		return fmt.Errorf("missing device-id value")
	}
	if value := values[refs["device-name"]]; value != nil {
		dev.deviceName = string(value)
	} else {
		return fmt.Errorf("missing device-name value")
	}
	if value := values[refs["app-name"]]; value != nil {
		dev.appName = string(value)
	} else {
		return fmt.Errorf("missing app-name value")
	}
	if value := values[refs["app-version"]]; value != nil {
		dev.appVersion = string(value)
	} else {
		return fmt.Errorf("missing app-version value")
	}

	s.logf("%s metadata device-name=%q app=%q version=%q", dev.label(), dev.deviceName, dev.appName, dev.appVersion)
	return nil
}

func (s *Server) nextID() string {
	id := atomic.AddUint64(&s.seq, 1)
	return fmt.Sprintf("device-%d", id)
}

func (s *Server) lookup(id string) *connectedDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.devices[id]
}

func (s *Server) store(dev *connectedDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if old := s.devices[dev.id]; old != nil && old != dev {
		old.channel.Close()
	}
	s.devices[dev.id] = dev
}

func (s *Server) remove(dev *connectedDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if current := s.devices[dev.id]; current == dev {
		delete(s.devices, dev.id)
	}
}

func (s *Server) snapshot() []Description {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]Description, 0, len(s.devices))
	for _, dev := range s.devices {
		devices = append(devices, Description{
			ID:         dev.id,
			DeviceID:   dev.deviceID,
			DeviceName: dev.deviceName,
			AppName:    dev.appName,
			AppVersion: dev.appVersion,
			Connected:  dev.connected,
		})
	}

	slices.SortFunc(devices, func(a, b Description) int {
		return strings.Compare(a.ID, b.ID)
	})

	return devices
}

func (s *Server) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger(format, args...)
	}
}

func (d *connectedDevice) label() string {
	if d.deviceName != "" && d.id != d.deviceName {
		return fmt.Sprintf("[%s/%s]", d.id, d.deviceName)
	}
	return fmt.Sprintf("[%s]", d.id)
}
