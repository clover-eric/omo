package main

import "testing"

func TestInitURLUsesListenAddress(t *testing.T) {
	t.Setenv("OMO_INIT_URL_HOST", "")

	got := initURL("127.0.0.1:8080", "tok")
	want := "http://127.0.0.1:8080/init?token=tok"
	if got != want {
		t.Fatalf("initURL() = %q, want %q", got, want)
	}
}

func TestInitURLUsesConfiguredHostWithListenPort(t *testing.T) {
	t.Setenv("OMO_INIT_URL_HOST", "203.0.113.10")

	got := initURL("0.0.0.0:23456", "tok")
	want := "http://203.0.113.10:23456/init?token=tok"
	if got != want {
		t.Fatalf("initURL() = %q, want %q", got, want)
	}
}

func TestInitURLEscapesToken(t *testing.T) {
	t.Setenv("OMO_INIT_URL_HOST", "https://ops.example.com:28080")

	got := initURL("0.0.0.0:23456", "tok+/=")
	want := "http://ops.example.com:28080/init?token=tok%2B%2F%3D"
	if got != want {
		t.Fatalf("initURL() = %q, want %q", got, want)
	}
}
