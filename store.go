package main

// store.go — persistent storage (single JSON file, mutex-protected).
// No external deps: stdlib only.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TransportCfg struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority"`
}

type Key struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Secret           string         `json:"secret,omitempty"`
	Context          string         `json:"context,omitempty"`
	Codec            string         `json:"codec"`
	Mode             string         `json:"mode"` // l3 | l4 | stream
	Transports       []TransportCfg `json:"transports"`
	DirectPort       int            `json:"direct_port,omitempty"`
	TrafficLimit     int64          `json:"traffic_limit"` // bytes, 0 = unlimited
	TrafficUp        int64          `json:"traffic_up"`
	TrafficDown      int64          `json:"traffic_down"`
	LastUp           uint64         `json:"last_up"`
	LastDown         uint64         `json:"last_down"`
	IPLimit          int            `json:"ip_limit"` // 0 = unlimited
	Expiry           string         `json:"expiry,omitempty"` // RFC3339 or ""
	Enabled          bool           `json:"enabled"`
	Status           string         `json:"status"` // active | disabled | limited | expired | error
	LimitedAt        string         `json:"limited_at,omitempty"` // когда ключ ушёл в limited/expired
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
	OnlineIPs        []string       `json:"online_ips,omitempty"`
	Connected        bool           `json:"connected"`
	ActiveTransport  string         `json:"active_transport,omitempty"`
	UptimeMs         int64          `json:"uptime_ms"`
	Error            string         `json:"error,omitempty"`
	LastSeen         string         `json:"last_seen,omitempty"`
}

type Settings struct {
	ShareHost       string `json:"share_host"`
	DirectFrom      int    `json:"direct_from"`
	DirectTo        int    `json:"direct_to"`
	OpenfluxBin     string `json:"openflux_bin"`
	DefaultMode     string `json:"default_mode"`
	DefaultCodec    string `json:"default_codec"`
	AutoDisable     bool   `json:"auto_disable"`
	AutoDeleteDays  int    `json:"auto_delete_days"` // 0 = выкл: удалять ключи спустя N дней в limited/expired
	PollIntervalSec int    `json:"poll_interval_sec"`
}

type Sample struct {
	Ts    int64  `json:"ts"`
	KeyID string `json:"key_id"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
}

type StoreData struct {
	AdminUser string            `json:"admin_user"`
	PassHash  string            `json:"pass_hash"`
	Salt      string            `json:"salt"`
	Settings  Settings          `json:"settings"`
	Keys      map[string]*Key   `json:"keys"`
	History   []Sample          `json:"history"`
}

type Store struct {
	mu   sync.Mutex
	path string
	d    StoreData
}

func defaultSettings() Settings {
	return Settings{
		ShareHost:       "",
		DirectFrom:      20000,
		DirectTo:        21000,
		OpenfluxBin:     "./openflux",
		DefaultMode:     "l3",
		DefaultCodec:    "batched",
		AutoDisable:     true,
		PollIntervalSec: 10,
	}
}

func NewStore(dir string) (*Store, error) {
	path := filepath.Join(dir, "panel.json")
	s := &Store{path: path}
	s.d.Keys = map[string]*Key{}
	s.d.History = []Sample{}
	s.d.Settings = defaultSettings()
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &s.d); err != nil {
			return nil, err
		}
		if s.d.Keys == nil {
			s.d.Keys = map[string]*Key{}
		}
		if s.d.History == nil {
			s.d.History = []Sample{}
		}
	}
	if s.d.Settings.DirectFrom == 0 {
		s.d.Settings = defaultSettings()
	}
	return s, nil
}

func (s *Store) save() error {
	s.d.History = trimHistory(s.d.History)
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func trimHistory(h []Sample) []Sample {
	// keep last 7 days of 5-min points max (~2016/key) + global cap 20000
	if len(h) > 20000 {
		return h[len(h)-20000:]
	}
	cut := time.Now().Add(-8 * 24 * time.Hour).Unix()
	i := 0
	for i < len(h) && h[i].Ts < cut {
		i++
	}
	if i > 0 {
		return append([]Sample{}, h[i:]...)
	}
	return h
}

// with runs fn under lock and saves afterwards if fn returns true.
func (s *Store) with(save bool, fn func(d *StoreData)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.d)
	if save {
		_ = s.save()
	}
}

func (s *Store) snapshot() StoreData {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.d
	keys := map[string]*Key{}
	for k, v := range s.d.Keys {
		nv := *v
		keys[k] = &nv
	}
	cp.Keys = keys
	h := make([]Sample, len(s.d.History))
	copy(h, s.d.History)
	cp.History = h
	return cp
}
