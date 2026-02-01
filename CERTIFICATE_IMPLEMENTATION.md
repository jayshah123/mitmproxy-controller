# Certificate Implementation Notes

Research and implementation details for CA certificate installation and trust verification across platforms.

## Reference Implementation: devcert

Analysis based on [davewasmer/devcert](https://github.com/davewasmer/devcert), a popular tool for generating locally-trusted development certificates.

## Platform Implementations

### macOS

#### System Keychain Installation

**devcert approach:**
```bash
sudo security add-trusted-cert \
  -d \                                    # Add to admin cert store
  -r trustRoot \                          # Mark as trusted root
  -k /Library/Keychains/System.keychain \ # Target System keychain
  -p ssl \                                # Trust for SSL
  -p basic \                              # Trust for basic policy
  <certificate-path>
```

**Our approach:**
```bash
# Delete existing certs first
while security delete-certificate -c mitmproxy /Library/Keychains/System.keychain 2>/dev/null; do :; done

# Import certificate
security import <cert-path> -k /Library/Keychains/System.keychain -t cert

# Set trust settings
security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain <cert-path>

# Refresh trust daemon
killall -HUP trustd 2>/dev/null || true
```

**Difference:** We do a two-step process (import + trust) and clean up existing certs first. devcert does a single `add-trusted-cert` command with policy flags (`-p ssl -p basic`).

#### Trust Verification

**Our approach (not in devcert):**
```bash
# Export cert from keychain
security find-certificate -c mitmproxy -p /Library/Keychains/System.keychain > temp.pem

# Verify trust
security verify-cert -c temp.pem
# Exit code 0 = trusted, non-zero = not trusted
```

devcert doesn't have explicit trust verification - it relies on installation success.

#### Certificate Locations

| Item | Path |
|------|------|
| mitmproxy CA cert | `~/.mitmproxy/mitmproxy-ca-cert.pem` |
| System Keychain | `/Library/Keychains/System.keychain` |
| User Keychain | `~/Library/Keychains/login.keychain-db` |

---

### Windows

#### Certificate Store Installation

**Both devcert and our approach:**
```cmd
certutil -addstore -user root <certificate-path>
```

**Flags:**
- `-addstore`: Add certificate to a store
- `-user`: Current user's certificate store (no admin required)
- `root`: Trusted Root Certification Authorities store

#### Trust Verification

On Windows, certificates in the Root store are inherently trusted. No separate verification needed:
```go
func isCertTrusted() bool {
    return isCertInstalled()
}
```

#### Check if Installed

```cmd
certutil -store -user Root
# Look for "mitmproxy" in output
```

#### Certificate Removal

```cmd
certutil -delstore -user root <cert-name>
```

#### Certificate Locations

| Item | Path |
|------|------|
| mitmproxy CA cert | `~/.mitmproxy/mitmproxy-ca-cert.cer` |
| Cert store | Registry: `HKCU\Software\Microsoft\SystemCertificates\Root` |

---

### Linux (for future reference)

devcert approach:
```bash
# Copy to system CA directory
sudo cp <cert-path> /usr/local/share/ca-certificates/devcert.crt

# Update system CA bundle
sudo update-ca-certificates
```

---

## Firefox Considerations

Firefox uses its own certificate store (NSS database) and does NOT trust system certificates by default.

### NSS certutil (different from Windows certutil)

```bash
# Install via Homebrew on macOS
brew install nss

# Add certificate to Firefox NSS database
certutil -A -d sql:~/.mozilla/firefox/<profile>/ -t "C,," -i <cert-path> -n "mitmproxy"
```

**Flags:**
- `-A`: Add certificate
- `-d sql:<dir>`: Database directory (modern NSS uses SQL format)
- `-t "C,,"`: Trust flags (C = valid CA)
- `-i`: Input certificate file
- `-n`: Nickname in database

### Firefox Database Locations

| Platform | Path |
|----------|------|
| macOS | `~/Library/Application Support/Firefox/Profiles/*.default*/cert9.db` |
| Windows | `%APPDATA%\Mozilla\Firefox\Profiles\*.default*\cert9.db` |
| Linux | `~/.mozilla/firefox/*.default*/cert9.db` |

### Current Limitation

Our implementation does NOT handle Firefox. Users must:
1. Open Firefox
2. Navigate to `about:preferences#privacy`
3. Click "View Certificates" → "Authorities" → "Import"
4. Import `~/.mitmproxy/mitmproxy-ca-cert.pem`
5. Check "Trust this CA to identify websites"

---

## Implementation Comparison

| Feature | devcert | mitmproxy-controller |
|---------|---------|---------------------|
| macOS System Keychain | ✓ | ✓ |
| macOS Trust Verification | ✗ (relies on install success) | ✓ (`security verify-cert`) |
| Windows Cert Store | ✓ | ✓ |
| Firefox (NSS) | ✓ (with fallback GUI) | ✗ |
| Chrome (uses system store) | ✓ | ✓ |
| Safari (uses system store) | ✓ | ✓ |
| Admin prompt (macOS) | ✓ (sudo) | ✓ (osascript) |
| Policy flags (`-p ssl`) | ✓ | ✗ |

---

## Security Considerations

### Private Key Protection

- **macOS/Linux**: File permissions (600), root ownership
- **Windows**: devcert uses AES-256 encryption for CA private keys

### Our Approach

mitmproxy generates and manages its own CA. We only install the public certificate to system trust stores. The private key remains in `~/.mitmproxy/` with default permissions.

---

## Future Improvements

1. **Add policy flags on macOS**: Use `-p ssl -p basic` for explicit trust scope
2. **Firefox support**: Detect Firefox profiles and install via NSS certutil
3. **Certificate removal**: Add menu option to uninstall/untrust certificate
4. **Linux support**: If adding Linux platform support

---

## References

- [devcert source](https://github.com/davewasmer/devcert)
- [Apple security command](https://ss64.com/osx/security.html)
- [Windows certutil](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/certutil)
- [NSS certutil](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS/tools/NSS_Tools_certutil)
- [mitmproxy certificates](https://docs.mitmproxy.org/stable/concepts-certificates/)
