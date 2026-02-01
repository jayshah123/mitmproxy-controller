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

**Our approach (updated):**
```bash
# Delete existing certs first
while security delete-certificate -c mitmproxy /Library/Keychains/System.keychain 2>/dev/null; do :; done

# Import certificate
security import <cert-path> -k /Library/Keychains/System.keychain -t cert

# Set trust settings with SSL policy (like devcert)
security add-trusted-cert -d -r trustRoot -p ssl -p basic -k /Library/Keychains/System.keychain <cert-path>

# Refresh trust daemon
killall -HUP trustd 2>/dev/null || true
```

#### Trust Verification

**Our approach:**
```bash
# Export cert from keychain
security find-certificate -c mitmproxy -p /Library/Keychains/System.keychain > temp.pem

# Verify trust with SSL policy
security verify-cert -c temp.pem -p ssl
# Exit code 0 = trusted, non-zero = not trusted
```

devcert doesn't have explicit trust verification - it relies on installation success. We go further by actually verifying the certificate is trusted for SSL.

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

#### Check if Installed (Thumbprint-based)

```cmd
# Get thumbprint of cert file
certutil -hashfile <cert-path> SHA1

# List certs in Root store and match thumbprint
certutil -store -user Root
# Parse "Cert Hash(sha1): xx xx xx..." and compare
```

Using thumbprint matching instead of name-based substring search ensures we identify the exact certificate.

#### Certificate Removal

```cmd
# Delete by thumbprint for precise targeting
certutil -delstore -user root <thumbprint>
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
| macOS Trust Verification | ✗ (relies on install success) | ✓ (`security verify-cert -p ssl`) |
| macOS Trust Action | ✓ | ✓ (`trustCACertificate()`) |
| macOS Remove Action | ✓ | ✓ (`removeCACertificate()`) |
| Windows Cert Store | ✓ | ✓ |
| Windows Thumbprint-based | ✓ | ✓ (`getCertThumbprint()`) |
| Windows Remove Action | ✓ | ✓ (`removeCACertificate()`) |
| Firefox (NSS) | ✓ (with fallback GUI) | ✗ |
| Chrome (uses system store) | ✓ | ✓ |
| Safari (uses system store) | ✓ | ✓ |
| Admin prompt (macOS) | ✓ (sudo) | ✓ (osascript) |
| Policy flags (`-p ssl`) | ✓ | ✓ |

---

## Issues Discovered & Fixes Applied

### Problem 1: UI Blocked Trust Action (macOS)

**Issue:** The menu item was disabled when a certificate was installed but not trusted. Users had no way to apply trust settings.

**Root cause:** `updateStatus()` set `mInstallCert.Disable()` for the "installed but not trusted" state.

**Fix:** Enable the menu item and change its label to "Trust CA Certificate". The click handler now checks state and calls `trustCACertificate()` instead of `installCACertificate()`.

### Problem 2: Keychain Mismatch (macOS)

**Issue:** `isCertInstalled()` checked multiple keychains (System + user default), but `isCertTrusted()` only checked System keychain. A cert could be "installed" in login keychain but show as "not trusted" because it wasn't in System keychain.

**Fix:** Changed `isCertInstalled()` to only check System keychain, matching `isCertTrusted()` behavior. Consistent keychain targeting across all functions.

### Problem 3: Missing SSL Policy in Verification (macOS)

**Issue:** `security verify-cert` was called without policy flags, which could give misleading trust results.

**Fix:** Added `-p ssl` flag to match the policy we use during installation (`-p ssl -p basic`).

### Problem 4: Name-based Certificate Matching (Windows)

**Issue:** `isCertInstalled()` used substring search for "mitmproxy" in `certutil -store` output. This is unreliable and could match unrelated certificates.

**Fix:** Implemented `getCertThumbprint()` to compute SHA1 hash of the cert file, then match against thumbprints in the store. Removal also uses thumbprint for precise targeting.

### Problem 5: No Remove Functionality

**Issue:** Users could install certificates but had no way to remove them via the app.

**Fix:** Added `removeCACertificate()` for both platforms:
- **macOS:** Removes trust settings with `security remove-trusted-cert -d`, then deletes cert objects with `security delete-certificate -c mitmproxy`
- **Windows:** Deletes by thumbprint with `certutil -delstore -user Root <thumbprint>`

### Problem 6: No Separate Trust Action

**Issue:** Only `installCACertificate()` existed, which does a full reinstall. No way to just apply trust to an already-installed cert.

**Fix:** Added `trustCACertificate()` for macOS that only runs `security add-trusted-cert` without reimporting. On Windows, trust is implicit with installation, so it just calls install.

---

## Security Considerations

### Private Key Protection

- **macOS/Linux**: File permissions (600), root ownership
- **Windows**: devcert uses AES-256 encryption for CA private keys

### Our Approach

mitmproxy generates and manages its own CA. We only install the public certificate to system trust stores. The private key remains in `~/.mitmproxy/` with default permissions.

---

## Future Improvements

1. **Firefox support**: Detect Firefox profiles and install via NSS certutil
2. **Linux support**: If adding Linux platform support

---

## References

- [devcert source](https://github.com/davewasmer/devcert)
- [Apple security command](https://ss64.com/osx/security.html)
- [Windows certutil](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/certutil)
- [NSS certutil](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS/tools/NSS_Tools_certutil)
- [mitmproxy certificates](https://docs.mitmproxy.org/stable/concepts-certificates/)
