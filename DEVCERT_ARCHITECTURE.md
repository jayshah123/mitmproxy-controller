# devcert Architecture Analysis

Deep dive into [davewasmer/devcert](https://github.com/davewasmer/devcert) - a Node.js library for generating locally-trusted development certificates.

## Overview

devcert creates a local Certificate Authority (CA), installs it in system trust stores, and generates domain-specific certificates signed by that CA. This enables HTTPS in local development without browser warnings.

## Core Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     certificateFor("example.com")                    │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                    ┌──────────────────────────┐
                    │   Root CA Installed?     │
                    └──────────────────────────┘
                         │              │
                        No             Yes
                         │              │
                         ▼              │
         ┌───────────────────────────┐  │
         │ installCertificateAuthority│  │
         │  1. Generate CA key/cert   │  │
         │  2. Install to trust stores│  │
         └───────────────────────────┘  │
                         │              │
                         └──────┬───────┘
                                │
                                ▼
                ┌──────────────────────────┐
                │ generateDomainCertificate │
                │  1. Create CSR            │
                │  2. Sign with CA          │
                │  3. Return key + cert     │
                └──────────────────────────┘
```

## Directory Structure

devcert stores all certificates in a platform-specific config directory:

```
~/.config/devcert/                    # macOS/Linux (XDG)
%LOCALAPPDATA%/devcert/               # Windows

├── certificate-authority/
│   ├── private-key.key               # Root CA private key (PROTECTED)
│   ├── certificate.cert              # Root CA public certificate
│   ├── serial                        # OpenSSL serial counter
│   └── index.txt                     # OpenSSL CA index database
│
├── domains/
│   ├── example.com/
│   │   ├── private-key.key           # Domain private key
│   │   ├── certificate.crt           # Domain certificate
│   │   └── certificate-signing-request.csr
│   │
│   └── san-<hash>/                   # Multi-domain (SAN) certificates
│       ├── private-key.key
│       ├── certificate.crt
│       └── ...
│
├── devcert-ca-version                # CA format version marker
└── .rnd                              # OpenSSL random seed
```

## Platform Abstraction

devcert uses a clean platform abstraction layer:

```
src/platforms/
├── darwin.ts      # macOS implementation
├── win32.ts       # Windows implementation
├── linux.ts       # Linux implementation
└── shared.ts      # Cross-platform utilities (NSS/Firefox)
```

Each platform implements a common interface:

```typescript
interface Platform {
  addToTrustStores(certPath: string, options?: Options): Promise<void>;
  removeFromTrustStores(certPath: string): void;
  readProtectedFile(filepath: string): Promise<string>;
  writeProtectedFile(filepath: string, contents: string): Promise<void>;
  deleteProtectedFiles(filepath: string): void;
}
```

---

## macOS Implementation

### Trust Store Installation

```typescript
async addToTrustStores(certificatePath: string): Promise<void> {
  // System trust store (Chrome, Safari, curl, etc.)
  run('sudo', [
    'security',
    'add-trusted-cert',
    '-d',                                    // Admin cert store
    '-r', 'trustRoot',                       // Trusted root CA
    '-k', '/Library/Keychains/System.keychain',
    '-p', 'ssl',                             // SSL policy
    '-p', 'basic',                           // Basic policy
    certificatePath
  ]);

  // Firefox (if certutil available)
  if (this.isFirefoxInstalled()) {
    if (commandExists('certutil')) {
      await addCertificateToNSSCertDB(
        this.FIREFOX_NSS_DIR,   // ~/Library/Application Support/Firefox/Profiles/*
        certificatePath,
        getCertUtilPath()
      );
    } else {
      // Fallback: open browser for manual install
      await openCertificateInFirefox(this.FIREFOX_BIN_PATH, certificatePath);
    }
  }
}
```

### Trust Store Removal

```typescript
removeFromTrustStores(certificatePath: string): void {
  // Remove from system keychain
  run('sudo', [
    'security',
    'remove-trusted-cert',
    '-d',
    certificatePath
  ]);

  // Remove from Firefox NSS
  if (commandExists('certutil')) {
    removeFromNSSCertDB(
      this.FIREFOX_NSS_DIR,
      'devcert',              // Certificate nickname
      getCertUtilPath()
    );
  }
}
```

### Protected File Access

macOS uses sudo + file permissions for CA key protection:

```typescript
async writeProtectedFile(filepath: string, contents: string) {
  if (exists(filepath)) {
    await run('sudo', ['rm', filepath]);
  }
  writeFile(filepath, contents);
  await run('sudo', ['chown', '0', filepath]);   // Root ownership
  await run('sudo', ['chmod', '600', filepath]); // Owner read/write only
}

async readProtectedFile(filepath: string) {
  return (await run('sudo', ['cat', filepath])).toString().trim();
}
```

---

## Windows Implementation

### Trust Store Installation

```typescript
async addToTrustStores(certificatePath: string): Promise<void> {
  // Windows system trust store (Chrome, Edge, IE, system apps)
  try {
    run('certutil', ['-addstore', '-user', 'root', certificatePath]);
  } catch (e) {
    e.output.map((buffer: Buffer) => {
      if (buffer) console.log(buffer.toString());
    });
  }

  // Firefox requires manual installation on Windows
  if (this.isFirefoxInstalled()) {
    await openCertificateInFirefox(this.FIREFOX_BIN_PATH, certificatePath);
  }
}
```

### Trust Store Removal

```typescript
removeFromTrustStores(certificatePath: string): void {
  try {
    run('certutil', ['-delstore', '-user', 'root', 'devcert']);
  } catch (e) {
    debug('failed to remove from Windows cert store, continuing...');
  }
}
```

### Protected File Access (Encrypted)

Windows uses AES-256 encryption instead of file permissions:

```typescript
private encryptionKey: string | null = null;

async writeProtectedFile(filepath: string, contents: string) {
  if (!this.encryptionKey) {
    this.encryptionKey = await UI.getWindowsEncryptionPassword();
  }
  const encrypted = this.encrypt(contents, this.encryptionKey);
  writeFile(filepath, encrypted);
}

async readProtectedFile(filepath: string): Promise<string> {
  if (!this.encryptionKey) {
    this.encryptionKey = await UI.getWindowsEncryptionPassword();
  }
  try {
    return this.decrypt(readFile(filepath, 'utf8'), this.encryptionKey);
  } catch (e) {
    if (e.message.includes('bad decrypt')) {
      this.encryptionKey = null;  // Clear bad password
      return this.readProtectedFile(filepath);  // Retry
    }
    throw e;
  }
}

private encrypt(text: string, key: string): string {
  const cipher = crypto.createCipher('aes256', Buffer.from(key));
  return cipher.update(text, 'utf8', 'hex') + cipher.final('hex');
}

private decrypt(encrypted: string, key: string): string {
  const decipher = crypto.createDecipher('aes256', Buffer.from(key));
  return decipher.update(encrypted, 'hex', 'utf8') + decipher.final('utf8');
}
```

---

## Linux Implementation

### Trust Store Installation

```typescript
async addToTrustStores(certificatePath: string): Promise<void> {
  // System CA certificates
  run('sudo', [
    'cp',
    certificatePath,
    '/usr/local/share/ca-certificates/devcert.crt'
  ]);
  run('sudo', ['update-ca-certificates']);

  // Chrome (uses NSS in ~/.pki/nssdb)
  if (this.isChromeInstalled()) {
    await addCertificateToNSSCertDB(
      this.CHROME_NSS_DIR,
      certificatePath,
      getCertUtilPath()
    );
  }

  // Firefox
  if (this.isFirefoxInstalled()) {
    await addCertificateToNSSCertDB(
      this.FIREFOX_NSS_DIR,
      certificatePath,
      getCertUtilPath()
    );
  }
}
```

---

## NSS (Firefox/Chrome) Support

### Shared NSS Utilities

```typescript
// Add certificate to NSS database
function addCertificateToNSSCertDB(
  nssDirGlob: string,
  certPath: string,
  certutilPath: string
): void {
  doForNSSCertDB(nssDirGlob, (dir, version) => {
    // Modern NSS uses SQL databases (cert9.db)
    // Legacy uses BerkeleyDB (cert8.db)
    const dirArg = version === 'modern' ? `sql:${dir}` : dir;
    
    run(certutilPath, [
      '-A',                    // Add certificate
      '-d', dirArg,            // Database directory
      '-t', 'C,,',             // Trust flags: C = valid CA
      '-i', certPath,          // Input certificate
      '-n', 'devcert'          // Nickname
    ]);
  });
}

// Remove certificate from NSS database
function removeFromNSSCertDB(
  nssDirGlob: string,
  nickname: string,
  certutilPath: string
): void {
  doForNSSCertDB(nssDirGlob, (dir, version) => {
    const dirArg = version === 'modern' ? `sql:${dir}` : dir;
    run(certutilPath, ['-D', '-d', dirArg, '-n', nickname]);
  });
}

// Iterate over NSS database directories
function doForNSSCertDB(
  nssDirGlob: string,
  callback: (dir: string, version: 'modern' | 'legacy') => void
): void {
  glob.sync(nssDirGlob).forEach((dir) => {
    if (exists(path.join(dir, 'cert9.db'))) {
      callback(dir, 'modern');
    } else if (exists(path.join(dir, 'cert8.db'))) {
      callback(dir, 'legacy');
    }
  });
}
```

### Firefox GUI Fallback

When NSS certutil is unavailable, devcert serves the certificate via HTTP and opens Firefox:

```typescript
async function openCertificateInFirefox(
  firefoxPath: string,
  certPath: string
): Promise<void> {
  const port = await getPort();
  
  const server = http.createServer((req, res) => {
    const { pathname } = url.parse(req.url!);
    
    if (pathname === '/certificate') {
      // Serve with correct MIME type triggers Firefox import dialog
      res.writeHead(200, { 'Content-type': 'application/x-x509-ca-cert' });
      res.write(readFile(certPath));
      res.end();
    } else {
      // Landing page with instructions
      res.writeHead(200, { 'Content-type': 'text/html' });
      res.write(`
        <html>
          <body>
            <h1>devcert Certificate Installation</h1>
            <p><a href="/certificate">Click here to install the certificate</a></p>
          </body>
        </html>
      `);
      res.end();
    }
  }).listen(port);
  
  // Open Firefox
  run(firefoxPath, [`http://localhost:${port}`]);
  
  // Wait for user, then cleanup
  await UI.waitForUserConfirmation();
  server.close();
}
```

---

## Certificate Generation

### Root CA Generation

```typescript
async function installCertificateAuthority(): Promise<void> {
  // Generate 2048-bit RSA private key
  openssl(['genrsa', '-out', rootCAKeyPath, '2048']);
  
  // Set restrictive permissions
  chmod(rootCAKeyPath, 400);
  
  // Generate self-signed CA certificate
  openssl([
    'req',
    '-config', opensslConfPath,
    '-key', rootCAKeyPath,
    '-out', rootCACertPath,
    '-new',
    '-subj', '/CN=devcert',
    '-x509',
    '-days', '825',              // ~2.25 years (macOS limit)
    '-extensions', 'v3_ca'
  ]);
  
  // Initialize OpenSSL CA database
  writeFile(caSerialPath, '01');
  writeFile(caDatabasePath, '');
  
  // Install to platform trust stores
  await currentPlatform.addToTrustStores(rootCACertPath);
}
```

### Domain Certificate Generation

```typescript
async function generateDomainCertificate(domain: string): Promise<{key: Buffer, cert: Buffer}> {
  const domainDir = path.join(domainsDir, domain);
  const keyPath = path.join(domainDir, 'private-key.key');
  const csrPath = path.join(domainDir, 'certificate-signing-request.csr');
  const certPath = path.join(domainDir, 'certificate.crt');
  
  mkdirp(domainDir);
  
  // Generate domain private key
  openssl(['genrsa', '-out', keyPath, '2048']);
  
  // Generate CSR
  openssl([
    'req',
    '-new',
    '-config', opensslConfPath,
    '-key', keyPath,
    '-out', csrPath,
    '-subj', `/CN=${domain}`
  ]);
  
  // Sign with CA
  openssl([
    'ca',
    '-config', opensslConfPath,
    '-in', csrPath,
    '-out', certPath,
    '-keyfile', rootCAKeyPath,
    '-cert', rootCACertPath,
    '-days', '825',
    '-batch',
    '-extensions', 'server_cert'
  ]);
  
  return {
    key: readFile(keyPath),
    cert: readFile(certPath)
  };
}
```

---

## Security Model

### Threat Model

| Threat | Mitigation |
|--------|------------|
| CA key theft | Protected files (sudo/encryption) |
| Unauthorized cert generation | CA key required for signing |
| Stale certificates | 825-day expiry (macOS requirement) |
| Browser bypass | Installs in all relevant trust stores |

### Key Protection by Platform

| Platform | Protection Method |
|----------|-------------------|
| macOS | `chmod 600`, `chown root`, requires sudo |
| Windows | AES-256 encryption with user password |
| Linux | `chmod 600`, `chown root`, requires sudo |

### Trust Boundaries

```
┌─────────────────────────────────────────────────────────┐
│                    User Space                            │
│  ┌─────────────────┐    ┌─────────────────────────────┐ │
│  │ Domain Certs    │    │ Application (Node.js)       │ │
│  │ (unprotected)   │───▶│ Uses domain key/cert        │ │
│  └─────────────────┘    └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
                              │
                              │ Signs with
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   Protected Zone                         │
│  ┌─────────────────┐    ┌─────────────────────────────┐ │
│  │ Root CA Key     │    │ System Trust Stores         │ │
│  │ (encrypted/600) │    │ (requires admin/sudo)       │ │
│  └─────────────────┘    └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## API Design

### Primary API

```typescript
// Generate certificate for a single domain
const { key, cert } = await devcert.certificateFor('localhost');

// Generate certificate for multiple domains (SAN)
const { key, cert } = await devcert.certificateFor(['localhost', 'my-app.local']);

// Options
const { key, cert } = await devcert.certificateFor('localhost', {
  skipHostsFile: false,    // Don't modify /etc/hosts
  skipCertutil: false,     // Skip Firefox NSS installation
  ui: customUI             // Custom UI for prompts
});
```

### Utility Functions

```typescript
// Check if CA is already installed
devcert.hasCertificateFor('localhost');

// Get paths to existing certificates
devcert.configuredDomains();

// Remove all devcert certificates and CA
devcert.uninstall();
```

---

## Key Takeaways

1. **Platform abstraction is essential** - Each OS has completely different trust store mechanisms

2. **Firefox is special** - Uses NSS database, not system trust store. Requires separate handling.

3. **Protected file patterns differ** - macOS/Linux use file permissions, Windows uses encryption

4. **CA key is the crown jewel** - All security measures focus on protecting this file

5. **Graceful degradation** - When automated methods fail (e.g., no certutil), fall back to GUI

6. **825-day certificate limit** - macOS enforces this for trusted certificates since Catalina

7. **Trust policy flags matter** - `-p ssl -p basic` on macOS specifies what the cert is trusted for

---

## References

- [devcert GitHub](https://github.com/davewasmer/devcert)
- [Apple Keychain Services](https://developer.apple.com/documentation/security/keychain_services)
- [Windows Certificate Store](https://docs.microsoft.com/en-us/windows-hardware/drivers/install/local-machine-and-current-user-certificate-stores)
- [Mozilla NSS](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS)
- [OpenSSL CA](https://www.openssl.org/docs/man1.1.1/man1/ca.html)
