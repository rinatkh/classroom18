package handler

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"
)

const refreshCookiePath = "/auth"

type CookieConfig struct {
	Name   string
	Secure bool
}

func (c CookieConfig) Validate() error {
	cookie := &http.Cookie{Name: c.Name, Path: refreshCookiePath, Secure: c.Secure}
	if err := cookie.Valid(); err != nil {
		return errors.New("invalid refresh cookie name")
	}
	if strings.HasPrefix(c.Name, "__Host-") || (strings.HasPrefix(c.Name, "__Secure-") && !c.Secure) {
		return errors.New("cookie prefix incompatible with Path=/auth or Secure")
	}
	return nil
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, raw string, expires time.Time) {
	maxAge := int(math.Ceil(time.Until(expires).Seconds()))
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: h.cookie.Name, Value: raw,
		HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
		Path: refreshCookiePath, Expires: expires.UTC(), MaxAge: maxAge})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: h.cookie.Name, Value: "",
		HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
		Path: refreshCookiePath, Expires: time.Unix(1, 0).UTC(), MaxAge: -1})
}
