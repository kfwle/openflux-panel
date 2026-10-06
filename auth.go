package main

// auth.go — admin auth: salted iterative SHA-256, random session tokens.

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"
)

type session struct {
	exp time.Time
}

var (
	sessMu sync.Mutex
	sess   = map[string]session{}
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPass(salt, pass string) string {
	h := salt + "\x00" + pass
	for i := 0; i < 50000; i++ {
		sum := sha256.Sum256([]byte(h))
		h = hex.EncodeToString(sum[:])
	}
	return h
}

func checkPass(salt, pass, want string) bool {
	got := hashPass(salt, pass)
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func newSession() string {
	t := randHex(32)
	sessMu.Lock()
	sess[t] = session{exp: time.Now().Add(7 * 24 * time.Hour)}
	// gc
	for k, v := range sess {
		if time.Now().After(v.exp) {
			delete(sess, k)
		}
	}
	sessMu.Unlock()
	return t
}

func validSession(t string) bool {
	sessMu.Lock()
	defer sessMu.Unlock()
	s, ok := sess[t]
	if !ok {
		return false
	}
	if time.Now().After(s.exp) {
		delete(sess, t)
		return false
	}
	return true
}

func dropSession(t string) {
	sessMu.Lock()
	delete(sess, t)
	sessMu.Unlock()
}
