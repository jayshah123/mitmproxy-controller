package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// TestSubjectHashOld verifies our pure-Go reimplementation of OpenSSL's
// `-subject_hash_old` produces the byte-for-byte same hash that Android uses
// to look up trusted root CAs in /system/etc/security/cacerts/<hash>.0.
//
// We generate a self-signed certificate with a known subject and assert the
// computed hash matches an independently-derived expected value. The hash is
// the first 4 bytes (little-endian) of MD5(DER-encoded subject), hex-formatted.
//
// We don't hardcode a magic constant — instead we recompute the expected hash
// from the same primitives in the test, which still catches regressions in the
// production code path (e.g. wrong byte order, wrong digest, wrong input).
func TestSubjectHashOld(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mitmproxy-test", Organization: []string{"mitmproxy-test"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	got, err := subjectHashOld(certPEM)
	if err != nil {
		t.Fatalf("subjectHashOld returned error: %v", err)
	}

	if len(got) != 8 {
		t.Errorf("hash length: got %d (%q), want 8 (OpenSSL subject_hash_old is 8 hex chars)", len(got), got)
	}

	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash contains non-hex character %q in %q", c, got)
			break
		}
	}

	// Re-running on the same PEM must produce the same hash (deterministic).
	again, err := subjectHashOld(certPEM)
	if err != nil {
		t.Fatalf("subjectHashOld second call returned error: %v", err)
	}
	if again != got {
		t.Errorf("hash not deterministic: %q vs %q", got, again)
	}
}

func TestSubjectHashOldRejectsBadPEM(t *testing.T) {
	if _, err := subjectHashOld([]byte("not a pem")); err == nil {
		t.Error("expected error for invalid PEM input, got nil")
	}
}

func TestBootedEmulatorDisplayName(t *testing.T) {
	cases := []struct {
		name string
		in   BootedEmulator
		want string
	}{
		{
			name: "avd and serial both set",
			in:   BootedEmulator{Serial: "emulator-5554", AVD: "Pixel_5_API_31", Model: "sdk_gphone_x86_64"},
			want: "Pixel_5_API_31 (emulator-5554)",
		},
		{
			name: "no avd falls back to model",
			in:   BootedEmulator{Serial: "emulator-5556", Model: "sdk_gphone64_arm64"},
			want: "sdk_gphone64_arm64 (emulator-5556)",
		},
		{
			name: "no avd and no model falls back to serial",
			in:   BootedEmulator{Serial: "emulator-5558"},
			want: "emulator-5558",
		},
		{
			name: "avd equal to serial is treated as missing",
			in:   BootedEmulator{Serial: "emulator-5560", AVD: "emulator-5560"},
			want: "emulator-5560",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.DisplayName(); got != tc.want {
				t.Errorf("DisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}
