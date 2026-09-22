package config

import "testing"

func TestRemoteEndpointMustBeExplicitAndSecure(t *testing.T) {
	cfg := Config{Mode: "cloud", Cloud: CloudConfig{Token: "secret"}}
	if cfg.ValidateEndpoint() == nil {
		t.Fatal("cloud placeholder accepted")
	}
	cfg.Cloud.Address = "http://example.com"
	if cfg.ValidateEndpoint() == nil {
		t.Fatal("insecure remote URL accepted")
	}
	cfg.Cloud.Address = "https://homey.example/"
	if err := cfg.ValidateEndpoint(); err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL() != "https://homey.example" {
		t.Fatal("trailing slash retained")
	}
	cfg.Cloud.Address = "https://user:secret@homey.example"
	if cfg.ValidateEndpoint() == nil {
		t.Fatal("credentials in endpoint accepted")
	}
}
