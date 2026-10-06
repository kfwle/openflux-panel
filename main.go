package main

// main.go — HTTP server + REST API + static web UI.

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	store   *Store
	manager *Manager
	dataDir string
	version = "1.0.0"
)

//go:embed web
var embeddedWeb embed.FS

func main() {
	port := flag.String("port", "8080", "panel listen port")
	dir := flag.String("data", "./data", "data directory")
	flag.Parse()
	if e := os.Getenv("PANEL_PORT"); e != "" {
		*port = e
	}
	if e := os.Getenv("PANEL_DATA"); e != "" {
		*dir = e
	}
	dataDir = *dir
	os.MkdirAll(dataDir, 0755)

	var err error
	store, err = NewStore(dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	ensureAdmin()
	manager = NewManager(store, dataDir)
	stop := make(chan struct{})
	go manager.Loop(stop)
	go func() {
		time.Sleep(2 * time.Second)
		manager.Tick()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", hLogin)
	mux.HandleFunc("/api/logout", hLogout)
	mux.HandleFunc("/api/me", hMe)
	mux.HandleFunc("/api/setup-needed", hSetupNeeded)
	mux.HandleFunc("/api/change-password", hChangePassword)
	mux.HandleFunc("/api/stats", auth(hStats))
	mux.HandleFunc("/api/keys", auth(hKeys))
	mux.HandleFunc("/api/keys/", auth(hKeyOne))
	mux.HandleFunc("/api/parse-link", auth(hParseLink))
	mux.HandleFunc("/api/settings", auth(hSettings))
	mux.HandleFunc("/api/history", auth(hHistory))
	mux.HandleFunc("/api/public-ip", auth(hPublicIP))

	// static: ./web на диске (dev) -> web рядом с бинарём -> вшитый в бинарь (single-file)
	diskWeb := ""
	for _, cand := range []string{"./web", filepath.Join(filepath.Dir(os.Args[0]), "web")} {
		if st, err := os.Stat(filepath.Join(cand, "index.html")); err == nil && !st.IsDir() {
			diskWeb = cand
			break
		}
	}
	embFS, _ := fs.Sub(embeddedWeb, "web")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if diskWeb != "" {
			f := filepath.Join(diskWeb, p)
			if _, err := os.Stat(f); err != nil {
				f = filepath.Join(diskWeb, "index.html")
			}
			http.ServeFile(w, r, f)
			return
		}
		if _, err := fs.Stat(embFS, p); err != nil {
			p = "index.html"
		}
		http.ServeFileFS(w, r, embFS, p)
	})

	addr := "0.0.0.0:" + *port
	log.Printf("OpenFlux Panel v%s on http://%s (data=%s)", version, addr, dataDir)
	snap := store.snapshot()
	if snap.PassHash == "" {
		log.Printf("FIRST RUN: login admin / admin123 and change password!")
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}

func ensureAdmin() {
	snap := store.snapshot()
	if snap.AdminUser == "" {
		u := os.Getenv("ADMIN_USER")
		if u == "" {
			u = "admin"
		}
		p := os.Getenv("ADMIN_PASS")
		if p == "" {
			p = "admin123"
		}
		salt := randHex(16)
		store.with(true, func(d *StoreData) {
			d.AdminUser = u
			d.Salt = salt
			d.PassHash = hashPass(salt, p)
		})
	}
}

// ---------- helpers ----------

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("of_session")
		if err != nil || !validSession(c.Value) {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ---------- auth handlers ----------

func hSetupNeeded(w http.ResponseWriter, r *http.Request) {
	snap := store.snapshot()
	writeJSON(w, 200, map[string]bool{"needed": snap.PassHash == ""})
}

func hLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var b struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := readJSON(r, &b); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	snap := store.snapshot()
	if b.User != snap.AdminUser || !checkPass(snap.Salt, b.Pass, snap.PassHash) {
		writeJSON(w, 401, map[string]string{"error": "неверный логин или пароль"})
		return
	}
	t := newSession()
	http.SetCookie(w, &http.Cookie{Name: "of_session", Value: t, Path: "/", HttpOnly: true, MaxAge: 7 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func hLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("of_session"); err == nil {
		dropSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "of_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func hMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("of_session")
	if err != nil || !validSession(c.Value) {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	snap := store.snapshot()
	writeJSON(w, 200, map[string]string{"user": snap.AdminUser, "version": version})
}

func hChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var b struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := readJSON(r, &b); err != nil || len(b.New) < 6 {
		writeJSON(w, 400, map[string]string{"error": "пароль минимум 6 символов"})
		return
	}
	snap := store.snapshot()
	if !checkPass(snap.Salt, b.Old, snap.PassHash) {
		writeJSON(w, 400, map[string]string{"error": "старый пароль неверный"})
		return
	}
	salt := randHex(16)
	store.with(true, func(d *StoreData) {
		d.Salt = salt
		d.PassHash = hashPass(salt, b.New)
	})
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// ---------- stats ----------

func hStats(w http.ResponseWriter, r *http.Request) {
	snap := store.snapshot()
	totalUp, totalDown := int64(0), int64(0)
	online, active := 0, 0
	for _, k := range snap.Keys {
		totalUp += k.TrafficUp
		totalDown += k.TrafficDown
		if k.Enabled && k.Status == "active" {
			active++
		}
		if len(k.OnlineIPs) > 0 || k.Connected {
			online++
		}
	}
	// today from history
	todayStart := time.Now().Truncate(24 * time.Hour).Unix()
	tUp, tDown := int64(0), int64(0)
	for _, s := range snap.History {
		if s.Ts >= todayStart {
			tUp += s.Up
			tDown += s.Down
		}
	}
	writeJSON(w, 200, map[string]any{
		"keys_total": len(snap.Keys), "keys_active": active,
		"online": online,
		"total_up": totalUp, "total_down": totalDown,
		"today_up": tUp, "today_down": tDown,
		"bin": snap.Settings.OpenfluxBin,
	})
}

// ---------- keys ----------

var allowedTypes = map[string]bool{
	"yandex": true, "vyandex": true, "boards": true,
	"mailru": true, "cupsonline": true, "direct": true, "oneme": true,
}

type keyInput struct {
	Name         string         `json:"name"`
	Secret       string         `json:"secret"`
	Context      string         `json:"context"`
	Codec        string         `json:"codec"`
	Mode         string         `json:"mode"`
	Transports   []TransportCfg `json:"transports"`
	DirectPort   int            `json:"direct_port"`
	TrafficLimit int64          `json:"traffic_limit"`
	IPLimit      int            `json:"ip_limit"`
	Expiry       string         `json:"expiry"`
	Enabled      bool           `json:"enabled"`
}

func validateKeyInput(in *keyInput, isNew bool) (string, *OFConfig) {
	if strings.TrimSpace(in.Name) == "" {
		return "укажите имя ключа", nil
	}
	if in.Codec != "batched" && in.Codec != "legacy" {
		in.Codec = "batched"
	}
	if in.Mode != "l3" && in.Mode != "l4" && in.Mode != "stream" {
		in.Mode = "l3"
	}
	if len(in.Transports) == 0 {
		return "добавьте хотя бы один транспорт", nil
	}
	for _, t := range in.Transports {
		if !allowedTypes[t.Type] {
			return "неизвестный транспорт: " + t.Type, nil
		}
	}
	if in.Mode == "stream" {
		if len(in.Transports) != 1 || (in.Transports[0].Type != "cupsonline" && in.Transports[0].Type != "mailru") {
			return "stream-режим: ровно один транспорт cupsonline или mailru, без секрета", nil
		}
		in.Secret = ""
		in.Context = ""
		in.DirectPort = 0
		return "", nil
	}
	// tunnel mode: direct needs port; secret required
	hasDirect := false
	for _, t := range in.Transports {
		if t.Type == "direct" {
			hasDirect = true
		}
	}
	if hasDirect && in.DirectPort == 0 {
		return "direct-транспорт требует порт", nil
	}
	if in.Secret != "" && secretChars(in.Secret) < 16 {
		return "секрет минимум 16 символов", nil
	}
	if in.Secret == "" && (len(in.Transports) > 1 || hasDirect) {
		return "для мульти-транспорта и direct нужен секрет (мин. 16 символов)", nil
	}
	if in.Expiry != "" {
		if _, err := time.Parse(time.RFC3339, in.Expiry); err != nil {
			// try date only
			if t, err2 := time.Parse("2006-01-02", in.Expiry); err2 == nil {
				in.Expiry = t.Format(time.RFC3339)
			} else {
				return "неверный формат даты expiry (RFC3339)", nil
			}
		}
	}
	return "", nil
}

func hKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		snap := store.snapshot()
		list := []*Key{}
		for _, k := range snap.Keys {
			list = append(list, k)
		}
		// sort by created
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if list[j].CreatedAt < list[i].CreatedAt {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
		out := []map[string]any{}
		for _, k := range list {
			link, cfg, ctx, lerr := KeyLink(k, snap.Settings.ShareHost)
			m := keyToMap(k)
			m["link"] = link
			m["link_config"] = cfg
			m["link_context"] = ctx
			if lerr != nil {
				m["link_error"] = lerr.Error()
			}
			out = append(out, m)
		}
		writeJSON(w, 200, out)
	case "POST":
		var in keyInput
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad json"})
			return
		}
		in.Enabled = true
		if msg, _ := validateKeyInput(&in, true); msg != "" {
			writeJSON(w, 400, map[string]string{"error": msg})
			return
		}
		snap := store.snapshot()
		// port allocation
		if in.DirectPort == 0 {
			for _, t := range in.Transports {
				if t.Type == "direct" {
					in.DirectPort = manager.AllocatePort()
					break
				}
			}
		} else {
			for _, k := range snap.Keys {
				if k.DirectPort == in.DirectPort {
					writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("порт %d уже занят", in.DirectPort)})
					return
				}
			}
		}
		if strings.TrimSpace(in.Secret) == "" && in.Mode != "stream" {
			// auto-generate readable secret
			in.Secret = randHex(16) // 32 hex chars
		}
		now := time.Now().UTC().Format(time.RFC3339)
		k := &Key{
			ID: randHex(8), Name: strings.TrimSpace(in.Name),
			Secret: in.Secret, Context: strings.TrimSpace(in.Context),
			Codec: in.Codec, Mode: in.Mode, Transports: in.Transports,
			DirectPort: in.DirectPort, TrafficLimit: in.TrafficLimit,
			IPLimit: in.IPLimit, Expiry: in.Expiry, Enabled: true,
			Status: "active", CreatedAt: now, UpdatedAt: now,
		}
		// validate link builds
		if _, _, _, lerr := KeyLink(k, snap.Settings.ShareHost); lerr != nil {
			writeJSON(w, 400, map[string]string{"error": "ссылка не собирается: " + lerr.Error()})
			return
		}
		store.with(true, func(d *StoreData) { d.Keys[k.ID] = k })
		go manager.Tick()
		writeJSON(w, 200, keyToMap(k))
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func keyToMap(k *Key) map[string]any {
	return map[string]any{
		"id": k.ID, "name": k.Name, "secret": k.Secret, "context": k.Context,
		"codec": k.Codec, "mode": k.Mode, "transports": k.Transports,
		"direct_port": k.DirectPort, "traffic_limit": k.TrafficLimit,
		"traffic_up": k.TrafficUp, "traffic_down": k.TrafficDown,
		"ip_limit": k.IPLimit, "expiry": k.Expiry, "enabled": k.Enabled,
		"status": k.Status, "created_at": k.CreatedAt, "online_ips": k.OnlineIPs,
		"connected": k.Connected, "active_transport": k.ActiveTransport,
		"uptime_ms": k.UptimeMs, "error": k.Error, "last_seen": k.LastSeen,
	}
}

func hKeyOne(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/keys/")
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	snap := store.snapshot()
	k, ok := snap.Keys[id]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "ключ не найден"})
		return
	}
	switch {
	case action == "" && r.Method == "PUT":
		var in keyInput
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad json"})
			return
		}
		if msg, _ := validateKeyInput(&in, false); msg != "" {
			writeJSON(w, 400, map[string]string{"error": msg})
			return
		}
		// port conflict
		for oid, ok2 := range snap.Keys {
			if oid != id && ok2.DirectPort == in.DirectPort && in.DirectPort != 0 {
				writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("порт %d уже занят", in.DirectPort)})
				return
			}
		}
		needRestart := false
		store.with(true, func(d *StoreData) {
			ck := d.Keys[id]
			if ck.Secret != in.Secret || ck.Mode != in.Mode || ck.Codec != in.Codec ||
				ck.DirectPort != in.DirectPort || transportsChanged(ck.Transports, in.Transports) ||
				ck.Context != strings.TrimSpace(in.Context) {
				needRestart = true
			}
			ck.Name = strings.TrimSpace(in.Name)
			ck.Secret = in.Secret
			ck.Context = strings.TrimSpace(in.Context)
			ck.Codec = in.Codec
			ck.Mode = in.Mode
			ck.Transports = in.Transports
			ck.DirectPort = in.DirectPort
			ck.TrafficLimit = in.TrafficLimit
			ck.IPLimit = in.IPLimit
			ck.Expiry = in.Expiry
			ck.Enabled = in.Enabled
			ck.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if !in.Enabled {
				ck.Status = "disabled"
			} else if ck.Status == "disabled" {
				ck.Status = "active"
				ck.LimitedAt = ""
				ck.Error = ""
			}
		})
		if needRestart {
			manager.stopKey(id)
		}
		go manager.Tick()
		snap2 := store.snapshot()
		writeJSON(w, 200, keyToMap(snap2.Keys[id]))
	case action == "" && r.Method == "DELETE":
		manager.deleteKeyFiles(id)
		store.with(true, func(d *StoreData) { delete(d.Keys, id) })
		writeJSON(w, 200, map[string]string{"ok": "1"})
	case action == "restart" && r.Method == "POST":
		manager.stopKey(id)
		store.with(true, func(d *StoreData) {
			if ck, ok := d.Keys[id]; ok {
				ck.LastUp = 0
				ck.LastDown = 0
				if ck.Enabled {
					ck.Status = "active"
					ck.Error = ""
				}
			}
		})
		go manager.Tick()
		writeJSON(w, 200, map[string]string{"ok": "1"})
	case action == "reset-traffic" && r.Method == "POST":
		store.with(true, func(d *StoreData) {
			if ck, ok := d.Keys[id]; ok {
				ck.TrafficUp = 0
				ck.TrafficDown = 0
				ck.LastUp = 0
				ck.LastDown = 0
				if ck.Status == "limited" {
					ck.Status = "active"
					ck.LimitedAt = ""
					ck.Error = ""
					ck.Enabled = true
				}
			}
		})
		go manager.Tick()
		writeJSON(w, 200, map[string]string{"ok": "1"})
	case action == "link" && r.Method == "GET":
		link, cfg, ctx, lerr := KeyLink(k, snap.Settings.ShareHost)
		if lerr != nil {
			writeJSON(w, 400, map[string]string{"error": lerr.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"link": link, "config": cfg, "context": ctx})
	case action == "logs" && r.Method == "GET":
		b, _ := os.ReadFile(filepath.Join(dataDir, "logs", "key_"+id+".log"))
		if len(b) > 20000 {
			b = b[len(b)-20000:]
		}
		writeJSON(w, 200, map[string]string{"logs": string(b)})
	default:
		writeJSON(w, 404, map[string]string{"error": "unknown action"})
	}
}

func transportsChanged(a, b []TransportCfg) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

// ---------- parse link ----------

func hParseLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var b struct {
		Link string `json:"link"`
	}
	if err := readJSON(r, &b); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	cfg, err := DecodeLink(b.Link)
	if err != nil {
		code := "bad"
		param := ""
		if le, ok := err.(*LinkError); ok {
			code = le.Code
			param = le.Param
		}
		writeJSON(w, 400, map[string]string{"error": err.Error(), "code": code, "param": param})
		return
	}
	ctx := DeriveContext(cfg.Context, "", cfg.Transports)
	writeJSON(w, 200, map[string]any{"config": cfg, "context": ctx})
}

// ---------- settings ----------

func hSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		snap := store.snapshot()
		writeJSON(w, 200, snap.Settings)
		return
	}
	var in Settings
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	if in.DirectFrom <= 0 || in.DirectTo <= in.DirectFrom || in.DirectTo > 65535 {
		writeJSON(w, 400, map[string]string{"error": "неверный диапазон портов"})
		return
	}
	if in.DefaultMode != "l3" && in.DefaultMode != "l4" {
		in.DefaultMode = "l3"
	}
	if in.PollIntervalSec < 3 {
		in.PollIntervalSec = 10
	}
	if in.AutoDeleteDays < 0 {
		in.AutoDeleteDays = 0
	}
	store.with(true, func(d *StoreData) { d.Settings = in })
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func hHistory(w http.ResponseWriter, r *http.Request) {
	snap := store.snapshot()
	writeJSON(w, 200, snap.History)
}

func hPublicIP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"ip": PublicIP()})
}
