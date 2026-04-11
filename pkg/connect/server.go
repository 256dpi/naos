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
	"time"

	"github.com/256dpi/naos/pkg/msg"
)

type connectedDevice struct {
	channel *msg.Channel
	desc    Description
}

// Server implements a NAOS Connect server.
type Server struct {
	upgrader *WebSocketUpgrader
	logger   func(string, ...any)
	token    string
	seq      uint64
	devices  sync.Map
}

// NewServer returns a new NAOS Connect server.
func NewServer() *Server {
	return &Server{
		upgrader: NewWebSocketUpgrader(nil),
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
	// check authorization
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// route request
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/list":
		s.handleList(w, r)
	case r.URL.Path == "/connect":
		s.handleConnect(w, r)
	case strings.HasPrefix(r.URL.Path, "/attach/"):
		s.handleDevice(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	// check if token is set
	if s.token == "" {
		return true
	}

	// check query parameter
	if r.URL.Query().Get("token") == s.token {
		return true
	}

	// check header
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	auth = strings.TrimPrefix(auth, "Bearer ")
	auth = strings.TrimSpace(auth)
	if auth == s.token {
		return true
	}

	return false
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	// collect descriptions
	descriptions := make([]Description, 0)
	for _, d := range s.devices.Range {
		dev := d.(*connectedDevice)
		desc := dev.desc
		desc.AttachURL = s.attachURL(r, dev.desc.DeviceID)
		descriptions = append(descriptions, desc)
	}

	// sort descriptions
	slices.SortFunc(descriptions, func(a, b Description) int {
		return strings.Compare(a.DeviceID, b.DeviceID)
	})

	// encode descriptions
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listResponse{
		Devices: descriptions,
	})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	// upgrade connection
	conn, err := s.upgrader.Upgrade(w, r)
	if err != nil || conn == nil {
		return
	}

	// create channel
	channel := msg.NewChannel(NewTransport(conn), nil, 10)

	// create device
	dev := &connectedDevice{
		channel: channel,
		desc: Description{
			Connected: time.Now(),
		},
	}

	// log
	s.logf("[%s] connected", conn.RemoteAddr())

	// identify device
	if !s.retryIdentify(dev) {
		s.logf("[%s] identification failed", conn.RemoteAddr())
		channel.Close()
		return
	}

	// set UUID to device ID
	dev.desc.UUID = dev.desc.DeviceID

	// log
	s.logf("[%s] id=%q name=%q app=%q version=%q", dev.desc.DeviceID, dev.desc.DeviceID, dev.desc.DeviceName, dev.desc.AppName, dev.desc.AppVersion)

	// close and replace stale device with same ID
	if old, loaded := s.devices.Swap(dev.desc.DeviceID, dev); loaded {
		old.(*connectedDevice).channel.Close()
		s.logf("[%s] replaced stale connection", dev.desc.DeviceID)
	}

	// await disconnect
	<-channel.Done()

	// remove device only if it is still this instance
	s.devices.CompareAndDelete(dev.desc.DeviceID, dev)

	// log
	s.logf("[%s] device disconnected", dev.desc.DeviceID)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	// check method
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// determine device ID
	id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/attach/"))
	if err != nil {
		http.Error(w, "invalid device id", http.StatusBadRequest)
		return
	}

	// find device
	d, _ := s.devices.Load(id)
	dev, ok := d.(*connectedDevice)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// upgrade connection
	conn, err := s.upgrader.Upgrade(w, r)
	if err != nil || conn == nil {
		return
	}

	// create transport
	client := NewTransport(conn)

	// log
	s.logf("[%s] client attached: %s", dev.desc.DeviceID, conn.RemoteAddr().String())

	// bridge transport
	err = Bridge(client, dev.channel)
	if err != nil && !errors.Is(err, io.EOF) {
		s.logf("[%s] bridge error: %v", dev.desc.DeviceID, err)
	}

	// log
	s.logf("[%s] client detached: %s", dev.desc.DeviceID, conn.RemoteAddr().String())
}

func (s *Server) retryIdentify(dev *connectedDevice) bool {
	// prepare state
	backoff := 250 * time.Millisecond
	timeout := 1 * time.Second

	// try to identify multiple times
	for attempt := 1; attempt <= 4; attempt++ {
		// await backoff
		select {
		case <-dev.channel.Done():
			return false
		case <-time.After(backoff):
		}

		// attempt identify
		err := s.doIdentify(dev, timeout)
		if err == nil {
			return true
		}

		// increase backoff and timeout
		if backoff < 2*time.Second {
			backoff *= 2
		}
		if timeout < 8*time.Second {
			timeout *= 2
		}
	}

	return false
}

func (s *Server) doIdentify(dev *connectedDevice, timeout time.Duration) error {
	// open session
	session, err := msg.OpenSession(dev.channel, timeout)
	if err != nil {
		return err
	}
	defer func() {
		_ = session.End(timeout)
	}()

	// get params
	deviceId, err := msg.GetParam(session, "device-id", timeout)
	if err != nil {
		return err
	}
	deviceName, err := msg.GetParam(session, "device-name", timeout)
	if err != nil {
		return err
	}
	appName, err := msg.GetParam(session, "app-name", timeout)
	if err != nil {
		return err
	}
	appVersion, err := msg.GetParam(session, "app-version", timeout)
	if err != nil {
		return err
	}

	// check params
	if len(deviceId) == 0 {
		return fmt.Errorf("missing device id")
	} else if len(deviceName) == 0 {
		return fmt.Errorf("missing device name")
	} else if len(appName) == 0 {
		return fmt.Errorf("missing app name")
	} else if len(appVersion) == 0 {
		return fmt.Errorf("missing app version")
	}

	// assign params
	dev.desc.DeviceID = string(deviceId)
	dev.desc.DeviceName = string(deviceName)
	dev.desc.AppName = string(appName)
	dev.desc.AppVersion = string(appVersion)

	return nil
}

func (s *Server) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger(format, args...)
	}
}

func (s *Server) attachURL(r *http.Request, id string) string {
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		switch strings.ToLower(forwarded) {
		case "http", "ws":
			scheme = "ws"
		case "https", "wss":
			scheme = "wss"
		}
	}

	return (&url.URL{
		Scheme: scheme,
		Host:   r.Host,
		Path:   "/attach/" + url.PathEscape(id),
	}).String()
}
