package main

import (
	"bufio"
	"crypto/md5"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Special host alias inside the Android emulator that points to the host machine's loopback.
const androidEmulatorHostAlias = "10.0.2.2"

// In-memory cache of which emulators we've successfully installed the CA cert on
// during this app session. Survives until the app exits.
var emulatorCACertificateInstalled = map[string]bool{}

type BootedEmulator struct {
	Serial string // e.g. "emulator-5554"
	AVD    string // e.g. "Pixel_5_API_31"
	Model  string // e.g. "sdk_gphone_x86_64"
}

func (e BootedEmulator) DisplayName() string {
	switch {
	case e.AVD != "" && e.AVD != e.Serial:
		return fmt.Sprintf("%s (%s)", e.AVD, e.Serial)
	case e.Model != "":
		return fmt.Sprintf("%s (%s)", e.Model, e.Serial)
	default:
		return e.Serial
	}
}

// adbPath finds the adb binary on disk. Returns "" if not found.
func adbPath() string {
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}

	var candidates []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if root := os.Getenv(env); root != "" {
			candidates = append(candidates, filepath.Join(root, "platform-tools", adbBinaryName()))
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Library", "Android", "sdk", "platform-tools", adbBinaryName()))
		case "windows":
			if local := os.Getenv("LOCALAPPDATA"); local != "" {
				candidates = append(candidates, filepath.Join(local, "Android", "Sdk", "platform-tools", adbBinaryName()))
			}
			candidates = append(candidates, filepath.Join(home, "AppData", "Local", "Android", "Sdk", "platform-tools", adbBinaryName()))
		default:
			candidates = append(candidates, filepath.Join(home, "Android", "Sdk", "platform-tools", adbBinaryName()))
		}
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func adbBinaryName() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}

func adbAvailable() (string, error) {
	p := adbPath()
	if p == "" {
		return "", fmt.Errorf("adb not found in PATH or Android SDK install locations")
	}
	return p, nil
}

func runAdb(args ...string) ([]byte, error) {
	p, err := adbAvailable()
	if err != nil {
		return nil, err
	}
	return exec.Command(p, args...).CombinedOutput()
}

func listBootedEmulators() ([]BootedEmulator, error) {
	p := adbPath()
	if p == "" {
		// Treat missing adb as "no emulators" rather than an error so the
		// menu doesn't constantly show an error on machines without Android tooling.
		return nil, nil
	}

	out, err := exec.Command(p, "devices", "-l").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %s", strings.TrimSpace(string(out)))
	}

	var emulators []BootedEmulator
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		serial := fields[0]
		state := fields[1]
		if state != "device" {
			continue
		}
		if !strings.HasPrefix(serial, "emulator-") {
			// Only consider emulators, not physical devices.
			continue
		}

		em := BootedEmulator{Serial: serial}
		for _, f := range fields[2:] {
			if strings.HasPrefix(f, "model:") {
				em.Model = strings.TrimPrefix(f, "model:")
			}
		}
		em.AVD = emulatorAVDName(p, serial)
		emulators = append(emulators, em)
	}

	sort.Slice(emulators, func(i, j int) bool {
		if emulators[i].AVD != emulators[j].AVD {
			return emulators[i].AVD < emulators[j].AVD
		}
		return emulators[i].Serial < emulators[j].Serial
	})

	return emulators, nil
}

func emulatorAVDName(adb, serial string) string {
	out, err := exec.Command(adb, "-s", serial, "emu", "avd", "name").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "OK" {
			continue
		}
		return line
	}
	return ""
}

// subjectHashOld implements OpenSSL's `-subject_hash_old` algorithm: MD5 of the
// DER-encoded subject, first 4 bytes as little-endian uint32, formatted as 8 hex chars.
// Android's /system/etc/security/cacerts uses this filename convention: <hash>.0
func subjectHashOld(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("invalid PEM data")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse certificate: %w", err)
	}
	sum := md5.Sum(cert.RawSubject)
	val := binary.LittleEndian.Uint32(sum[:4])
	return fmt.Sprintf("%08x", val), nil
}

func installAndTrustCACertificateOnEmulator(serial string) string {
	if emulatorCACertificateInstalled[serial] {
		return fmt.Sprintf("CA certificate already installed on %s.", serial)
	}

	if _, err := adbAvailable(); err != nil {
		return err.Error()
	}

	certPath := getMitmproxyCertPath()
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "CA cert not found. Start mitmproxy first to generate it."
		}
		return fmt.Sprintf("Failed to read CA cert: %v", err)
	}

	hash, err := subjectHashOld(certPEM)
	if err != nil {
		return fmt.Sprintf("Failed to compute cert hash: %v", err)
	}
	target := fmt.Sprintf("/system/etc/security/cacerts/%s.0", hash)

	// 1. Restart adbd as root.
	if out, err := runAdb("-s", serial, "root"); err != nil {
		return fmt.Sprintf("adb root failed: %s", trimAdb(out, err))
	}

	// adb root restarts adbd; wait for it to reconnect.
	if out, err := runAdb("-s", serial, "wait-for-device"); err != nil {
		return fmt.Sprintf("wait-for-device failed: %s", trimAdb(out, err))
	}

	// 2. Remount /system as read-write.
	if out, err := runAdb("-s", serial, "remount"); err != nil {
		detail := trimAdb(out, err)
		return fmt.Sprintf("adb remount failed: %s. Emulator must be started with `-writable-system` and not be a Google Play image.", detail)
	}

	// 3. Push the certificate to the system trust store.
	if out, err := runAdb("-s", serial, "push", certPath, target); err != nil {
		return fmt.Sprintf("adb push failed: %s", trimAdb(out, err))
	}

	// 4. Fix permissions and SELinux context.
	if out, err := runAdb("-s", serial, "shell", "chmod", "644", target); err != nil {
		return fmt.Sprintf("chmod failed: %s", trimAdb(out, err))
	}

	emulatorCACertificateInstalled[serial] = true
	return fmt.Sprintf("CA certificate installed on %s (reboot the emulator if apps still fail).", serial)
}

func isEmulatorCACertificateKnownInstalled(serial string) bool {
	return emulatorCACertificateInstalled[serial]
}

func enableEmulatorProxy(serial string) string {
	if _, err := adbAvailable(); err != nil {
		return err.Error()
	}
	target := fmt.Sprintf("%s:%s", androidEmulatorHostAlias, proxyPort)
	if out, err := runAdb("-s", serial, "shell", "settings", "put", "global", "http_proxy", target); err != nil {
		return fmt.Sprintf("Failed to enable proxy on %s: %s", serial, trimAdb(out, err))
	}
	return fmt.Sprintf("Proxy enabled on %s (%s)", serial, target)
}

func disableEmulatorProxy(serial string) string {
	if _, err := adbAvailable(); err != nil {
		return err.Error()
	}
	// `settings put global http_proxy :0` is the documented way to clear the proxy.
	if out, err := runAdb("-s", serial, "shell", "settings", "put", "global", "http_proxy", ":0"); err != nil {
		return fmt.Sprintf("Failed to disable proxy on %s: %s", serial, trimAdb(out, err))
	}
	return fmt.Sprintf("Proxy disabled on %s", serial)
}

func isEmulatorProxyEnabled(serial string) bool {
	out, err := runAdb("-s", serial, "shell", "settings", "get", "global", "http_proxy")
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(out))
	if val == "" || val == "null" || val == ":0" {
		return false
	}
	return true
}

func rebootEmulator(serial string) string {
	if _, err := adbAvailable(); err != nil {
		return err.Error()
	}
	if out, err := runAdb("-s", serial, "reboot"); err != nil {
		return fmt.Sprintf("Failed to reboot %s: %s", serial, trimAdb(out, err))
	}
	// Clear the in-session cert cache so the menu re-offers install after the
	// emulator comes back (the cert may or may not survive depending on AVD type).
	delete(emulatorCACertificateInstalled, serial)
	return fmt.Sprintf("Reboot requested for %s", serial)
}

func trimAdb(out []byte, err error) string {
	detail := strings.TrimSpace(string(out))
	if detail == "" {
		return err.Error()
	}
	return detail
}
