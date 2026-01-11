package config

import (
	"testing"
)

func TestBaseURL(t *testing.T) {
	cfg := &Config{
		Host: "192.168.1.100",
		Port: 4859,
	}

	expected := "http://192.168.1.100:4859"
	if got := cfg.BaseURL(); got != expected {
		t.Errorf("BaseURL() = %q, want %q", got, expected)
	}
}

func TestBaseURLDefaultPort(t *testing.T) {
	cfg := &Config{
		Host: "localhost",
		Port: 80,
	}

	expected := "http://localhost:80"
	if got := cfg.BaseURL(); got != expected {
		t.Errorf("BaseURL() = %q, want %q", got, expected)
	}
}

func TestBaseURLWithTLS(t *testing.T) {
	cfg := &Config{
		Host: "10-0-1-1.homey.homeylocal.com",
		Port: 4860,
		TLS:  true,
	}

	expected := "https://10-0-1-1.homey.homeylocal.com:4860"
	if got := cfg.BaseURL(); got != expected {
		t.Errorf("BaseURL() = %q, want %q", got, expected)
	}
}

func TestBaseURLWithoutTLS(t *testing.T) {
	cfg := &Config{
		Host: "10.0.1.1",
		Port: 4859,
		TLS:  false,
	}

	expected := "http://10.0.1.1:4859"
	if got := cfg.BaseURL(); got != expected {
		t.Errorf("BaseURL() = %q, want %q", got, expected)
	}
}
