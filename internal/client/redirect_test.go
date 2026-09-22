package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fishfisher/homeyctl/internal/config"
)

func TestDoesNotFollowRedirects(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client := New(&config.Config{Mode: "local", Local: config.LocalConfig{Address: server.URL, Token: "secret"}})
	if _, err := client.GetFlows(); err == nil {
		t.Fatal("redirect treated as success")
	}
	if forwarded {
		t.Fatal("redirect followed")
	}
}
