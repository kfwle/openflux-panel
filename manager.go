package main

// manager.go — supervises one `openflux --role=exit` process per key,
// polls IPC Status for traffic, tracks IPs via `ss`, enforces limits.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StatusPayload struct {
	Running   bool     `json:"running"`
	Connected bool     `json:"connected"`
	BytesIn   uint64   `json:"bytes_in"`
	BytesOut  uint64   `json:"bytes_out"`
	UptimeMs  int64    `json:"uptime_ms"`
	Active    string   `json:"active,omitempty"`
	ActiveAll []string `json:"active_all,omitempty"`
}

type procInfo struct {
	cmd     *exec.Cmd
	ipcPath string
	pid     int
}

type Manager struct {
	store *Store
	dir   string
	mu    sync.Mutex
	procs map[string]*procInfo
}

func NewManager(store *Store, dir string) *Manager {
	m := &Manager{store: store, dir: dir, procs: map[string]*procInfo{}}
	os.MkdirAll(filepath.Join(dir, "run"), 0755)
	os.MkdirAll(filepath.Join(dir, "logs"), 0755)
	return m
}

func (m *Manager) binPath() string {
	snap := m.store.snapshot()
	if snap.Settings.OpenfluxBin != "" {
		return snap.Settings.OpenfluxBin
	}
	return "./openflux"
}

func keyFilePath(dir, id string) string { return filepath.Join(dir, "run", "key_"+id+".key") }
func ipcPath(dir, id string) string     { return filepath.Join(dir, "run", "ipc_"+id+".sock") }
func cookiePath(dir, id, ttype string) string {
	return filepath.Join(dir, "run", fmt.Sprintf("cookies_%s_%s.json", id, ttype))
}

// buildArgs constructs CLI args for exit process of key.
func (m *Manager) buildArgs(k *Key) []string {
	globalURL := ""
	// global url = highest-priority doc url (for context compat)
	best := -1
	for i, t := range k.Transports {
		if t.Type == "cupsonline" || t.Type == "direct" || t.Type == "oneme" {
			continue
		}
		if t.URL == "" {
			continue
		}
		if best < 0 || t.Priority > k.Transports[best].Priority {
			best = i
		}
	}
	if best >= 0 {
		globalURL = k.Transports[best].URL
	}
	if k.Context != "" {
		globalURL = k.Context
	}

	args := []string{"--role=exit", "--mode=" + k.Mode}
	if k.Codec != "" {
		args = append(args, "--codec="+k.Codec)
	}
	// transports spec
	parts := []string{}
	hasDirect := false
	for _, t := range k.Transports {
		pr := t.Priority
		if pr == 0 {
			pr = 50
		}
		parts = append(parts, fmt.Sprintf("%s:%d", t.Type, pr))
		if t.Type == "direct" {
			hasDirect = true
		}
	}
	negotiate := len(parts) > 1 || hasDirect || k.Secret != ""
	if k.Mode == "stream" {
		negotiate = false
	}
	if len(parts) == 1 && !hasDirect && !negotiate {
		// single classic transport
		args = append(args, "--transport="+k.Transports[0].Type)
	} else if len(parts) > 0 {
		args = append(args, "--transports="+strings.Join(parts, ","))
	}
	if negotiate {
		args = append(args, "--negotiate")
	}
	if k.Secret != "" && k.Mode != "stream" {
		args = append(args, "--encryption-key-file="+keyFilePath(m.dir, k.ID))
	}
	if globalURL != "" {
		args = append(args, "--url="+globalURL)
	}
	for _, t := range k.Transports {
		switch t.Type {
		case "yandex":
			if t.URL != "" {
				args = append(args, "--yandex-url="+t.URL)
			}
		case "vyandex":
			if t.URL != "" {
				args = append(args, "--vyandex-url="+t.URL)
			}
		case "boards":
			if t.URL != "" {
				args = append(args, "--boards-url="+t.URL)
			}
		case "mailru":
			if t.URL != "" {
				args = append(args, "--mailru-url="+t.URL)
			}
		case "cupsonline":
			if t.URL != "" {
				args = append(args, "--cupsonline-url="+t.URL)
			}
		case "oneme":
			// token/uid cannot come from link; panel stores "token|uid" in URL field
			if t.URL != "" {
				if tk, uid, ok := strings.Cut(t.URL, "|"); ok {
					args = append(args, "--oneme-token="+tk, "--oneme-uid="+uid)
				}
			}
		}
	}
	if hasDirect && k.DirectPort != 0 {
		args = append(args, fmt.Sprintf("--direct-listen=0.0.0.0:%d", k.DirectPort))
	}
	args = append(args, "--ipc-socket="+ipcPath(m.dir, k.ID))
	// per-transport cookie store: use first doc type
	ctype := "yandex"
	for _, t := range k.Transports {
		if t.Type != "direct" && t.Type != "oneme" {
			ctype = t.Type
			break
		}
	}
	args = append(args, "--cookie-store="+cookiePath(m.dir, k.ID, ctype))
	return args
}

func (m *Manager) ensureKeyFile(k *Key) error {
	if k.Secret == "" || k.Mode == "stream" {
		return nil
	}
	return os.WriteFile(keyFilePath(m.dir, k.ID), []byte(k.Secret), 0600)
}

func (m *Manager) isRunning(id string) bool {
	m.mu.Lock()
	p, ok := m.procs[id]
	m.mu.Unlock()
	if !ok || p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	// signal 0 check via /proc
	if _, err := os.FindProcess(p.cmd.Process.Pid); err != nil {
		return false
	}
	// check if process exited
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		return false
	}
	return true
}

func (m *Manager) startKey(k *Key) error {
	if m.isRunning(k.ID) {
		return nil
	}
	bin := m.binPath()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("openflux binary not found: %s", bin)
	}
	if err := m.ensureKeyFile(k); err != nil {
		return err
	}
	args := m.buildArgs(k)
	// remove stale ipc socket
	os.Remove(ipcPath(m.dir, k.ID))
	cmd := exec.Command(bin, args...)
	logF, err := os.OpenFile(filepath.Join(m.dir, "logs", "key_"+k.ID+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		logF.Close()
		return err
	}
	pid := cmd.Process.Pid
	m.mu.Lock()
	m.procs[k.ID] = &procInfo{cmd: cmd, ipcPath: ipcPath(m.dir, k.ID), pid: pid}
	m.mu.Unlock()
	go func() {
		cmd.Wait()
		logF.Close()
	}()
	return nil
}

func (m *Manager) stopKey(id string) {
	m.mu.Lock()
	p, ok := m.procs[id]
	if ok {
		delete(m.procs, id)
	}
	m.mu.Unlock()
	if ok && p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	os.Remove(ipcPath(m.dir, id))
}

// readStatus dials IPC socket, reads frames, returns latest Status.
func readStatus(ipcSock string) (*StatusPayload, error) {
	conn, err := net.DialTimeout("unix", ipcSock, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	var last *StatusPayload
	buf := make([]byte, 0, 64<<10)
	tmp := make([]byte, 32 << 10)
	// read whatever server pushes (Status every second); take ~3.5s window max one status
	deadline := time.Now().Add(1200 * time.Millisecond)
	_ = conn.SetDeadline(deadline)
	for {
		n, rerr := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			// try parse frames
			for len(buf) >= 5 {
				nlen := binary.BigEndian.Uint32(buf[:4])
				if nlen == 0 || nlen > 1<<20 {
					break
				}
				if len(buf) < 4+int(nlen) {
					break
				}
				typ := buf[4]
				payload := buf[5 : 4+nlen]
				buf = buf[4+nlen:]
				if typ == 0x03 {
					var st StatusPayload
					if json.Unmarshal(payload, &st) == nil {
						last = &st
					}
				}
			}
			if last != nil {
				return last, nil
			}
		}
		if rerr != nil {
			break
		}
		if time.Now().After(deadline) {
			break
		}
	}
	if last != nil {
		return last, nil
	}
	return nil, fmt.Errorf("no status")
}

// onlineIPsForPort parses `ss -tnH` for established connections to port.
func onlineIPsForPort(port int) []string {
	if port == 0 {
		return nil
	}
	out, err := exec.Command("ss", "-tnH").Output()
	if err != nil {
		return nil
	}
	want := ":" + strconv.Itoa(port)
	set := map[string]bool{}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		f := bytes.Fields(line)
		if len(f) < 5 {
			continue
		}
		state := string(f[0])
		if state != "ESTAB" {
			continue
		}
		// ss output: State Recv-Q Send-Q Local:port Peer:port
		local := string(f[3])
		peer := string(f[4])
		if !strings.HasSuffix(local, want) {
			continue
		}
		ip := peer
		// strip [v6]:port or ip:port
		if i := strings.LastIndex(ip, ":"); i >= 0 {
			ip = ip[:i]
		}
		ip = strings.Trim(ip, "[]")
		if ip != "" {
			set[ip] = true
		}
	}
	ips := make([]string, 0, len(set))
	for ip := range set {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips
}

func expired(expiry string) bool {
	if expiry == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

// Tick runs one supervision cycle.
func (m *Manager) Tick() {
	snap := m.store.snapshot()
	now := time.Now().UTC().Format(time.RFC3339)
	changed := false
	var samples []Sample
	for _, k := range snap.Keys {
		ck := *k
		// expiry / limits state
		if expired(ck.Expiry) {
			if ck.Status != "expired" || ck.Enabled {
				m.stopKey(ck.ID)
				ck.Enabled = false
				ck.Status = "expired"
				ck.Connected = false
				ck.Error = "срок действия истёк"
				changed = true
				m.store.with(true, func(d *StoreData) {
					d.Keys[ck.ID] = &ck
				})
			}
			continue
		}
		if ck.TrafficLimit > 0 && ck.TrafficUp+ck.TrafficDown >= ck.TrafficLimit {
			if ck.Status != "limited" {
				if snap.Settings.AutoDisable {
					m.stopKey(ck.ID)
					ck.Enabled = false
				}
				ck.Status = "limited"
				ck.Connected = false
				ck.Error = "лимит трафика исчерпан"
				changed = true
				m.store.with(true, func(d *StoreData) {
					d.Keys[ck.ID] = &ck
				})
			}
			continue
		}
		if !ck.Enabled {
			if ck.Status != "disabled" && ck.Status != "limited" && ck.Status != "expired" {
				ck.Status = "disabled"
				m.store.with(true, func(d *StoreData) { d.Keys[ck.ID] = &ck })
			}
			m.stopKey(ck.ID)
			continue
		}
		// enabled: ensure process
		if !m.isRunning(ck.ID) {
			if err := m.startKey(&ck); err != nil {
				ck.Status = "error"
				ck.Error = err.Error()
				ck.Connected = false
				m.store.with(true, func(d *StoreData) { d.Keys[ck.ID] = &ck })
				continue
			}
			ck.Status = "active"
			ck.Error = ""
			changed = true
		}
		// poll IPC
		st, serr := readStatus(ipcPath(m.dir, ck.ID))
		if serr == nil && st != nil {
			dUp := int64(0)
			dDown := int64(0)
			if st.BytesOut >= ck.LastUp {
				dUp = int64(st.BytesOut - ck.LastUp)
			} else {
				dUp = int64(st.BytesOut) // process restarted, counters reset
			}
			if st.BytesIn >= ck.LastDown {
				dDown = int64(st.BytesIn - ck.LastDown)
			} else {
				dDown = int64(st.BytesIn)
			}
			ck.LastUp = st.BytesOut
			ck.LastDown = st.BytesIn
			ck.TrafficUp += dUp
			ck.TrafficDown += dDown
			ck.Connected = st.Connected
			ck.ActiveTransport = st.Active
			ck.UptimeMs = st.UptimeMs
			ck.LastSeen = now
			if dUp+dDown > 0 {
				samples = append(samples, Sample{Ts: time.Now().Unix(), KeyID: ck.ID, Up: dUp, Down: dDown})
			}
			// traffic limit hit right now?
			if ck.TrafficLimit > 0 && ck.TrafficUp+ck.TrafficDown >= ck.TrafficLimit {
				if snap.Settings.AutoDisable {
					m.stopKey(ck.ID)
					ck.Enabled = false
				}
				ck.Status = "limited"
				ck.Connected = false
				ck.Error = "лимит трафика исчерпан"
			} else if ck.Status != "active" {
				ck.Status = "active"
				ck.Error = ""
			}
			changed = true
		} else {
			ck.Connected = false
		}
		// IP tracking (direct only)
		hasDirect := false
		for _, t := range ck.Transports {
			if t.Type == "direct" {
				hasDirect = true
			}
		}
		if hasDirect && ck.DirectPort != 0 {
			ips := onlineIPsForPort(ck.DirectPort)
			ck.OnlineIPs = ips
			if ck.IPLimit > 0 && len(ips) > ck.IPLimit {
				ck.Error = fmt.Sprintf("превышен лимит IP: %d > %d", len(ips), ck.IPLimit)
				if snap.Settings.AutoDisable {
					m.stopKey(ck.ID)
					ck.Enabled = false
					ck.Status = "limited"
					ck.Connected = false
				}
				changed = true
			}
		}
		ck.UpdatedAt = now
		m.store.with(false, func(d *StoreData) {
			d.Keys[ck.ID] = &ck
		})
		changed = true
	}
	if changed {
		m.store.with(true, func(d *StoreData) {})
	}
	// aggregate samples into 5-min history points
	if len(samples) > 0 {
		m.store.with(true, func(d *StoreData) {
			byKey := map[string]*Sample{}
			for _, s := range samples {
				if e, ok := byKey[s.KeyID]; ok {
					e.Up += s.Up
					e.Down += s.Down
				} else {
					cp := s
					byKey[s.KeyID] = &cp
				}
			}
			slot := time.Now().Truncate(5 * time.Minute).Unix()
			for keyID, s := range byKey {
				merged := false
				for i := len(d.History) - 1; i >= 0 && i >= len(d.History)-50; i-- {
					if d.History[i].KeyID == keyID && d.History[i].Ts == slot {
						d.History[i].Up += s.Up
						d.History[i].Down += s.Down
						merged = true
						break
					}
				}
				if !merged {
					d.History = append(d.History, Sample{Ts: slot, KeyID: keyID, Up: s.Up, Down: s.Down})
				}
			}
		})
	}
}

// Loop runs Tick periodically.
func (m *Manager) Loop(stop <-chan struct{}) {
	interval := 10 * time.Second
	snap := m.store.snapshot()
	if snap.Settings.PollIntervalSec >= 3 {
		interval = time.Duration(snap.Settings.PollIntervalSec) * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			m.Tick()
		}
	}
}

// AllocatePort finds free direct port in range.
func (m *Manager) AllocatePort() int {
	snap := m.store.snapshot()
	used := map[int]bool{}
	for _, k := range snap.Keys {
		if k.DirectPort != 0 {
			used[k.DirectPort] = true
		}
	}
	for p := snap.Settings.DirectFrom; p <= snap.Settings.DirectTo; p++ {
		if !used[p] {
			return p
		}
	}
	return 0
}

// PublicIP tries to detect first public IPv4.
func PublicIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 3*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return ""
}
