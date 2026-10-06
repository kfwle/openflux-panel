package main

// oflink.go — openflux:// link encoder/decoder.
// Byte-for-byte compatible with OpenFlux share package and OpenFluxAndroid:
//   openflux://v1/<base64url_nopad(raw_DEFLATE(JSON(Config)))>
// JSON fields: name, negotiate, codec, secret, context, mode,
// transports[{type,name,url,priority,dial}]

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const linkPrefix = "openflux://v1/"
const maxPayload = 16 << 10
const minSecretChars = 16
const contextPlaceholder = "http://#"

type OFTransport struct {
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Dial     string `json:"dial,omitempty"`
}

type OFConfig struct {
	Name       string        `json:"name,omitempty"`
	Negotiate  bool          `json:"negotiate,omitempty"`
	Codec      string        `json:"codec,omitempty"`
	Secret     string        `json:"secret,omitempty"`
	Context    string        `json:"context,omitempty"`
	Mode       string        `json:"mode,omitempty"`
	Transports []OFTransport `json:"transports"`
}

type LinkError struct {
	Code  string
	Param string
	Text  string
}

func (e *LinkError) Error() string { return e.Text }

func secretChars(s string) int { return len(utf16.Encode([]rune(s))) }

var knownLinkTypes = map[string]bool{
	"yandex": true, "vyandex": true, "boards": true,
	"mailru": true, "cupsonline": true, "direct": true,
}

func validateConfig(c *OFConfig) *LinkError {
	if len(c.Transports) == 0 {
		return &LinkError{"no_transports", "", "no transports"}
	}
	if c.Mode != "" {
		if c.Mode != "stream" {
			return &LinkError{"unknown_mode", c.Mode, fmt.Sprintf("unknown mode %q", c.Mode)}
		}
		if len(c.Transports) != 1 {
			return &LinkError{"stream_one_transport", "", "stream mode rides exactly one transport"}
		}
		t := c.Transports[0].Type
		if t != "cupsonline" && t != "mailru" {
			return &LinkError{"stream_transport", t, fmt.Sprintf("stream mode cannot ride %q", t)}
		}
		if c.Negotiate || c.Secret != "" {
			return &LinkError{"stream_plain_only", "", "stream mode has no session or secret"}
		}
	}
	if len(c.Transports) > 1 && !c.Negotiate {
		return &LinkError{"several_need_session", "", "several transports need a negotiated session"}
	}
	if c.Negotiate && secretChars(c.Secret) < minSecretChars {
		return &LinkError{"session_secret", strconv.Itoa(minSecretChars), "session needs a secret of at least 16 characters"}
	}
	if c.Secret != "" && secretChars(c.Secret) < minSecretChars {
		return &LinkError{"short_secret", strconv.Itoa(minSecretChars), "secret must be at least 16 characters"}
	}
	if c.Codec != "" && c.Codec != "batched" && c.Codec != "legacy" {
		return &LinkError{"unknown_codec", c.Codec, fmt.Sprintf("unknown codec %q", c.Codec)}
	}
	for _, t := range c.Transports {
		if !knownLinkTypes[t.Type] {
			if t.Type == "oneme" {
				return &LinkError{"not_shareable", t.Type, "MAX (oneme) cannot be shared"}
			}
			return &LinkError{"unknown_transport", t.Type, fmt.Sprintf("unknown transport %q", t.Type)}
		}
		if t.Type == "direct" {
			if t.Dial == "" {
				return &LinkError{"direct_no_dial", "", "direct needs the exit address"}
			}
			if !c.Negotiate {
				return &LinkError{"direct_needs_session", "", "direct only works in a negotiated session"}
			}
		}
	}
	return nil
}

// EncodeConfig validates and encodes config into openflux:// link.
func EncodeConfig(c OFConfig) (string, error) {
	if err := validateConfig(&c); err != nil {
		return "", err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(raw); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return linkPrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func normalizeBody(b string) string {
	b = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n', ' ', '​':
			return -1
		case '+':
			return '-'
		case '/':
			return '_'
		}
		return r
	}, b)
	return strings.TrimRight(b, "=")
}

// DecodeLink parses and validates an openflux:// link (lenient like core).
func DecodeLink(link string) (OFConfig, error) {
	var zero OFConfig
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, linkPrefix) {
		if strings.HasPrefix(strings.ToLower(link), "openflux://") && !strings.HasPrefix(link, "openflux://") {
			return zero, &LinkError{"case_changed", "", "link letters changed case; copy it again"}
		}
		if strings.HasPrefix(link, "openflux://") {
			return zero, &LinkError{"unsupported_version", "", "unsupported link version; update OpenFlux"}
		}
		return zero, &LinkError{"not_link", "", "not an openflux:// link"}
	}
	body := normalizeBody(strings.TrimPrefix(link, linkPrefix))
	packed, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return zero, &LinkError{"damaged", "", "bad link encoding (truncated or mangled?)"}
	}
	raw, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(packed)), maxPayload+1))
	if err != nil {
		return zero, &LinkError{"damaged", "", "bad link payload (not raw DEFLATE?)"}
	}
	if len(raw) > maxPayload {
		return zero, &LinkError{"too_large", "", "link payload too large"}
	}
	var c OFConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return zero, &LinkError{"bad_payload", "", "bad link payload"}
	}
	if verr := validateConfig(&c); verr != nil {
		return zero, verr
	}
	return c, nil
}

func contextURL(t OFTransport) bool {
	switch t.Type {
	case "cupsonline", "direct", "oneme":
		return false
	}
	return t.URL != "" && t.URL != contextPlaceholder
}

// DeriveContext mirrors transport.KDFContexts primary rule.
func DeriveContext(explicit, globalURL string, ts []OFTransport) string {
	if explicit != "" {
		return explicit
	}
	cups := len(ts) == 1 && ts[0].Type == "cupsonline"
	if globalURL != "" && globalURL != contextPlaceholder && !cups {
		return globalURL
	}
	best := -1
	for i, s := range ts {
		if !contextURL(s) {
			continue
		}
		if best < 0 || s.Priority > ts[best].Priority {
			best = i
		}
	}
	if best >= 0 {
		return ts[best].URL
	}
	return contextPlaceholder
}

// MakeLink normalizes like share.Make then encodes.
func MakeLink(c OFConfig) (OFConfig, string, string, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Context = strings.TrimSpace(c.Context)
	if c.Codec == "batched" || c.Mode == "stream" {
		c.Codec = ""
	}
	ts := make([]OFTransport, len(c.Transports))
	for i, t := range c.Transports {
		t.Type = strings.TrimSpace(t.Type)
		t.Name = strings.TrimSpace(t.Name)
		t.URL = strings.TrimSpace(t.URL)
		t.Dial = strings.TrimSpace(t.Dial)
		if t.Name == t.Type {
			t.Name = ""
		}
		ts[i] = t
	}
	if len(ts) == 1 {
		ts[0].Priority = 0
	}
	c.Transports = ts
	if c.Secret == "" {
		c.Context = ""
	} else if c.Context == "" {
		c.Context = DeriveContext("", "", c.Transports)
	}
	link, err := EncodeConfig(c)
	if err != nil {
		return c, "", "", err
	}
	ctx := ""
	if c.Mode != "stream" {
		ctx = DeriveContext(c.Context, "", c.Transports)
	}
	return c, link, ctx, nil
}

var (
	ipCacheMu sync.Mutex
	ipCache   string
	ipCacheAt time.Time
)

// cachedPublicIP returns the server's outward IP, cached for 5 minutes
// so link generation never blocks on repeated lookups.
func cachedPublicIP() string {
	ipCacheMu.Lock()
	defer ipCacheMu.Unlock()
	if ipCache != "" && time.Since(ipCacheAt) < 5*time.Minute {
		return ipCache
	}
	if ip := PublicIP(); ip != "" {
		ipCache = ip
		ipCacheAt = time.Now()
	}
	return ipCache
}

// BuildConfigFromKey builds share.Config + context for a panel key.
func BuildConfigFromKey(k *Key, shareHost string) (OFConfig, string) {
	cfg := OFConfig{
		Name:      k.Name,
		Negotiate: false,
		Codec:     k.Codec,
		Secret:    k.Secret,
		Context:   k.Context,
		Transports: []OFTransport{},
	}
	if k.Mode == "stream" {
		cfg.Mode = "stream"
		cfg.Secret = ""
		cfg.Context = ""
		cfg.Negotiate = false
		for _, t := range k.Transports {
			cfg.Transports = append(cfg.Transports, OFTransport{Type: t.Type, URL: t.URL})
		}
		return cfg, ""
	}
	for _, t := range k.Transports {
		ot := OFTransport{Type: t.Type, URL: t.URL, Priority: t.Priority}
		if t.Type == "direct" {
			host := shareHost
			if host == "" {
				host = cachedPublicIP()
			}
			if host == "" {
				host = "YOUR_SERVER_IP"
			}
			ot.Dial = fmt.Sprintf("%s:%d", host, k.DirectPort)
		}
		cfg.Transports = append(cfg.Transports, ot)
	}
	if len(cfg.Transports) > 1 || hasDirect(cfg.Transports) {
		cfg.Negotiate = true
	} else if cfg.Secret != "" {
		cfg.Negotiate = true
	}
	norm, _, ctx, _ := MakeLink(cfg)
	_ = norm
	return cfg, ctx
}

func hasDirect(ts []OFTransport) bool {
	for _, t := range ts {
		if t.Type == "direct" {
			return true
		}
	}
	return false
}

// KeyLink returns final link + normalized config + context for key.
func KeyLink(k *Key, shareHost string) (link string, cfg OFConfig, ctx string, err error) {
	raw, _ := BuildConfigFromKey(k, shareHost)
	return makeLinkRaw(raw)
}

func makeLinkRaw(c OFConfig) (string, OFConfig, string, error) {
	norm, link, ctx, err := MakeLink(c)
	return link, norm, ctx, err
}
