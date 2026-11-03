package main

import "testing"

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("SERVICE_TEMPLATE_TEST_VALUE", "configured")
	if got := envOrDefault("SERVICE_TEMPLATE_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("envOrDefault() = %q, want configured", got)
	}
	t.Setenv("SERVICE_TEMPLATE_TEST_VALUE", "")
	if got := envOrDefault("SERVICE_TEMPLATE_TEST_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault() = %q, want fallback", got)
	}
}
