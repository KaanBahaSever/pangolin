# Pangolin — Architecture

Pangolin is a local-first, single-user password keeper for Windows, macOS and Linux.
It has no server, no account, no telemetry and makes no network connections.

This document is the source of truth for the design. The code in this repository
implements it; where the code and this document disagree, that is a bug.

- [1. Goals and non-goals](#1-goals-and-non-goals)
- [2. Review of the legacy plan](#2-review-of-the-legacy-plan)
- [3. Threat model](#3-threat-model)
- [4. System overview](#4-system-overview)
- [5. Cryptographic design](#5-cryptographic-design)
- [6. Storage](#6-storage)
- [7. Memory hygiene](#7-memory-hygiene)
- [8. User experience](#8-user-experience)
- [9. Project layout](#9-project-layout)
- [10. Testing](#10-testing)
- [11. Known limitations and roadmap](#11-known-limitations-and-roadmap)

---

## 1. Goals and non-goals

**Goals**

1. Nothing readable on disk. A stolen laptop, a synced backup or a copied vault
   folder reveals nothing without the master password.
2. Zero knowledge, locally. The master password is never stored, in any form.
   There is no recovery path and no backdoor.
3. Boring, standard cryptography. Only well-reviewed primitives and libraries;
   no home-made constructions.
4. Secrets stay encrypted until the moment they are used, and are forgotten
   as soon as possible afterwards.
5. A calm interface: one window, search first, two clicks to copy a password.

**Non-goals (for now)**

- Sync, sharing, browser integration and mobile clients.
- Defending against malware already running as the same user while the vault
  is unlocked (see [threat model](#3-threat-model)).

---

## 2. Review of the legacy plan

The original notes described a Go desktop app with a launcher screen, a
"new password" form and a "my passwords" list. The intent was right (local,
cross-platform, dark theme); the design was not safe to build as written.

### Security flaws

| # | Legacy decision | Problem | Replacement |
|---|---|---|---|
| S1 | No master password or unlock step anywhere in the plan | Anyone with access to the machine or the file has every credential | Master password gates everything; vault is locked at start (§5, §8) |
| S2 | Storage is "JSON, SQLite **or** an encrypted file" | Encryption is optional and unspecified; two of the three options are plaintext | Mandatory encryption at two layers: SQLCipher plus per-field AEAD (§5, §6) |
| S3 | No key derivation specified | Any naive choice (raw password as key, a single hash) is brute-forceable on a GPU | Argon2id, 256 MiB, parameters stored with the vault so they can be raised (§5.2) |
| S4 | No integrity protection | A modified file is silently accepted | Every page and every secret field is authenticated; tampering fails closed (§5) |
| S5 | Data lives in "temporary memory" and is written "when the window closes, so it does not stay in memory" | A crash, kill or power loss loses all changes; writing to disk does not remove anything from RAM; every secret is plaintext in RAM for the whole session | Each change is committed in a transaction immediately; secrets are decrypted only on demand (§6.3, §7) |
| S6 | The list screen shows username/password pairs | Shoulder surfing, screen sharing, screenshots | Passwords are masked, never rendered in the list, and revealed one at a time (§8) |
| S7 | No lock, no idle timeout, no clipboard handling | An unattended session or clipboard leaks secrets indefinitely | Manual lock, idle auto-lock, clipboard auto-clear (§7) |
| S8 | No threat model | No way to judge whether any decision is adequate | §3 |

### UX and product flaws

| # | Legacy decision | Problem | Replacement |
|---|---|---|---|
| U1 | An entry is only "username or e-mail" + "password" | Entries cannot be told apart: there is no site or title | Title, username, URL, password, notes |
| U2 | Home screen with two large buttons that open further windows | An extra click before every task, and several windows to manage | One window: search + list + detail |
| U3 | A "reset" icon on the home screen with no defined meaning | A destructive, ambiguous action one click away from the main screen | Removed. Destroying a vault is done by deleting its folder |
| U4 | No search, copy, delete or generator | The four things a password manager is used for most | All four are first-class (§8) |
| U5 | Save only on exit | Users cannot tell whether their data is safe | Saved on every change; no "save on close" step exists |
| U6 | Grey piggy bank on a white background, `logo.png` placeholder | Clashes with a dark UI; a piggy bank is something you smash to open | Vector logo: a curled pangolin as a padlock (`assets/logo.svg`) |

### Unnecessary complexity and open ends

- "Fyne or Wails" was left undecided. Decided: **Fyne** (§4.1).
- "A new window per screen" adds window-lifecycle code for no user benefit.
- An `onClose` save hook is a critical code path that only runs at exit and is
  therefore rarely exercised. It is removed entirely.
- The notes embedded a code-generation prompt and a personal name. Neither
  belongs in an architecture document.

---

## 3. Threat model

### Assets

The master password, the vault keys, and the stored passwords and notes.
Titles, usernames and URLs are treated as sensitive metadata: encrypted at
rest, but held in memory while the vault is unlocked so that search works.

### In scope

| Threat | Defence |
|---|---|
| Theft or copy of the device, disk, vault folder or a backup | All data encrypted at rest; key derived with Argon2id |
| Offline guessing of the master password | Memory-hard KDF with a per-vault random salt; minimum password length |
| Tampering with the vault files | AEAD on the header and on each secret; HMAC on each database page |
| Swapping ciphertext between entries or fields | Entry ID and field name are bound as associated data |
| Unattended unlocked session | Idle auto-lock; manual lock; keys destroyed on lock |
| Clipboard left holding a password | Timed auto-clear, and clear on lock and on exit |
| Shoulder surfing | Masked by default; nothing secret in the list view |
| Secrets paged to swap or captured in a crash dump | Keys kept in locked, guarded memory; core dumps disabled |
| Data loss from a crash | Transactional writes; atomic header replacement |

### Out of scope

| Threat | Why |
|---|---|
| Malware or an attacker running as the same user while the vault is unlocked | It can read process memory, log keystrokes and read the screen. No user-space application can prevent this |
| A compromised operating system, firmware or hardware | Same reason |
| A weak master password | The KDF slows guessing; it cannot make a guessable password safe |
| Coercion | Not addressable in software |
| Forgetting the master password | By design, the data is then unrecoverable |

---

## 4. System overview

```
┌──────────────────────────────────────────────────────────┐
│ ui            Fyne widgets. Holds no keys.               │
├──────────────────────────────────────────────────────────┤
│ session       Idle timer, clipboard guard                │
├──────────────────────────────────────────────────────────┤
│ vault         Create / unlock / lock, entry CRUD,        │
│               field encryption, password change          │
├───────────────────────┬──────────────────────────────────┤
│ crypto                │ storage                          │
│ Argon2id, HKDF,       │ vault.header  (key slot, JSON)   │
│ XChaCha20-Poly1305,   │ vault.db      (SQLCipher)        │
│ locked key buffers    │                                  │
└───────────────────────┴──────────────────────────────────┘
```

Dependencies point downwards only. The UI never sees a key; it asks the vault
for a decrypted value and hands it to the screen or the clipboard.

### 4.1 Technology choices

| Concern | Choice | Reason |
|---|---|---|
| Language | Go | Memory safe, one static binary per platform, first-class `x/crypto` |
| GUI | Fyne | Native widgets drawn by the app itself. Wails was rejected: it puts secrets in a web view, which means a JavaScript heap, a DOM and a much larger attack surface |
| Database | SQLCipher | See §6.1 |
| KDF | Argon2id | Winner of the Password Hashing Competition, RFC 9106 |
| AEAD | XChaCha20-Poly1305 | 192-bit nonce is safe to generate at random; constant-time in software on every CPU |
| Key memory | `memguard` | `mlock`/`VirtualLock`, guard pages, wipe on destroy |

---

## 5. Cryptographic design

### 5.1 Key hierarchy

```
master password ──Argon2id(salt, t, m, p)──▶ KEK (32 bytes)
                                              │
                          XChaCha20-Poly1305  │  unwrap
                                              ▼
                                   master key MK (32 bytes, random)
                                              │
                        ┌── HKDF-SHA-256 ─────┴───── HKDF-SHA-256 ──┐
                        ▼ info "pangolin/v1/database"               ▼ info "pangolin/v1/fields"
                 database key (32)                           field key (32)
                        │                                           │
                 SQLCipher page encryption               XChaCha20-Poly1305 per secret
```

- **MK** is generated once, from the operating system CSPRNG, when the vault is
  created. It never leaves the process and is stored only in wrapped form.
- The password protects MK, not the data directly. Changing the master
  password re-wraps 32 bytes instead of re-encrypting the vault.
- Two independent sub-keys mean that exposing the database key (which must
  be handed to a C library, §7.3) does not expose stored passwords.

### 5.2 Key derivation

Argon2id with a 16-byte random salt and, by default:

| Parameter | Value |
|---|---|
| Memory | 256 MiB |
| Iterations | 4 |
| Parallelism | 4 |
| Output | 32 bytes |

That is four times the memory and one more pass than the second recommended
parameter set of RFC 9106 (64 MiB, 3 passes), and far above the OWASP minimum.
It costs a desktop a fraction of a second per unlock and an attacker 256 MiB
per guess. The parameters are written into the vault header, so they can be
raised later without breaking existing vaults. On load they are checked against
hard bounds, so a manipulated header cannot cause an out-of-memory condition
or a trivially weak derivation.

### 5.3 Vault header

`vault.header` is a small JSON file and the only thing stored unencrypted:

```json
{
  "format": "pangolin-vault",
  "version": 1,
  "kdf":  { "algorithm": "argon2id", "time": 4, "memory_kib": 262144, "threads": 4, "salt": "…" },
  "wrap": { "algorithm": "xchacha20-poly1305", "nonce": "…", "ciphertext": "…" }
}
```

`ciphertext` is MK sealed under the KEK. The associated data is a canonical
encoding of `format`, `version` and every KDF parameter, so none of them can be
altered without the unwrap failing.

A successful unwrap is the password check. No password hash or verifier is
stored. A wrong password and a corrupted header produce the same error.

### 5.4 Field encryption

Passwords and notes are sealed individually with XChaCha20-Poly1305 under the
field key:

```
blob = nonce (24 random bytes) ‖ ciphertext ‖ tag (16 bytes)
AAD  = "pangolin/v1/field" ‖ 0x00 ‖ entry id ‖ 0x00 ‖ field name
```

Binding the entry ID and field name means a ciphertext moved to another row
or column fails authentication.

### 5.5 Randomness

All keys, salts, nonces, entry IDs and generated passwords come from
`crypto/rand`. The generator uses rejection sampling, so there is no modulo bias.

---

## 6. Storage

### 6.1 Why SQLCipher

| Option | Verdict |
|---|---|
| Plain JSON or SQLite (legacy) | Rejected: plaintext |
| A custom single-file encrypted format | Workable, but every write rewrites the whole file, and crash safety, integrity and migrations would all be ours to get right |
| Encrypted key-value stores (Badger, Bolt with a wrapper) | Encryption is less widely reviewed; no relational queries |
| **SQLCipher** | **Chosen** |

SQLCipher is SQLite with transparent, page-level AES-256 encryption and a
per-page HMAC-SHA-512. It has been deployed for over a decade in products with
real adversaries (Signal among them), it inherits SQLite's transaction and
crash-recovery guarantees, the entire file including the schema is
encrypted, and it gives us ordinary SQL with schema migrations.

Pangolin does **not** use SQLCipher's built-in passphrase KDF (PBKDF2). It
supplies the raw 256-bit database key derived in §5.1, so the only
password-hashing function in the system is Argon2id.

### 6.2 Layout on disk

```
<user config dir>/Pangolin/
├── vault.header     key slot (§5.3)
└── vault.db         SQLCipher database
```

The directory is created with mode `0700` and files with `0600`. The location
can be overridden with `PANGOLIN_HOME`.

The three preferences (language, auto-lock delay, clipboard delay) are not
secret and live in the toolkit's ordinary per-user preferences file, outside
the vault, so they can be read before unlocking.

```sql
CREATE TABLE entries (
  id         TEXT PRIMARY KEY,      -- 128-bit random, hex
  title      TEXT NOT NULL,
  username   TEXT NOT NULL DEFAULT '',
  url        TEXT NOT NULL DEFAULT '',
  password   BLOB NOT NULL,         -- sealed (§5.4)
  notes      BLOB NOT NULL,         -- sealed (§5.4)
  created_at INTEGER NOT NULL,      -- unix seconds
  updated_at INTEGER NOT NULL
);
```

`PRAGMA user_version` carries the schema version. Connection settings:
`cipher_memory_security = ON`, `secure_delete = ON`, `temp_store = MEMORY`,
`journal_mode = DELETE`, `foreign_keys = ON`.

### 6.3 Durability

- Every create, update and delete is one committed transaction. There is no
  in-memory "dirty" state and nothing to save at exit.
- The header is replaced atomically: write to a temporary file, `fsync`,
  rename over the original.
- Creating a vault fails if one already exists; nothing is overwritten.

---

## 7. Memory hygiene

Go is garbage collected, which limits what can be promised. This section states
both what is done and what is not.

### 7.1 Keys

- KEK, MK and the field key live in `memguard` locked buffers: pinned in RAM
  (no swap), surrounded by guard pages, wiped when destroyed.
- The KEK is destroyed as soon as MK has been unwrapped.
- Core dumps are disabled at start-up where the platform allows it.
- Locking destroys every key buffer, closes the database (SQLCipher wipes its
  own key and page cache), clears the clipboard if Pangolin filled it, and
  replaces the UI with the unlock screen. Unlocking again requires the master
  password and a full Argon2id derivation.

### 7.2 Secrets

- Stored passwords and notes stay sealed in the database and in memory. They
  are decrypted only when the user reveals, copies or edits an entry.
- Decrypted values are returned as byte slices and wiped by the caller once
  handed to the screen or clipboard.
- The entry list holds titles, usernames and URLs only.

### 7.3 Clipboard

- Copying a secret starts a timer (default 20 seconds). When it fires, the
  clipboard is cleared only if it still holds what Pangolin put there.
- Pangolin remembers a SHA-256 of the copied value for that comparison, not
  the value.
- The clipboard is also cleared on lock and on exit.

### 7.4 Residual risks

These are real and are not hidden:

1. **GUI text.** The toolkit's text fields and the system clipboard take Go
   strings, which are immutable and cannot be wiped. A revealed or typed
   password may persist in freed heap memory until it is reused. This includes
   the master password as typed. The fields are cleared and dereferenced
   immediately, and a garbage collection is forced on lock, but this is
   mitigation, not a guarantee.
2. **Database key.** The SQLite driver accepts its key as a string. The
   database key therefore passes through unwipeable memory. This is why it is
   a separate sub-key: on its own it decrypts metadata, not passwords.
3. **Clipboard history.** Operating-system clipboard managers and cloud
   clipboard sync may keep a copy that Pangolin cannot remove.

---

## 8. User experience

One window, three states.

**Create** (first run). Master password, confirmation, a plain statement that
it cannot be recovered. Minimum 12 characters.

**Unlock.** One field. Enter unlocks. Errors do not say whether the password or
the file was at fault.

**Vault.**

```
┌─────────────────────────────────────────────────────────────┐
│ [ Search…                              ]   [+ New]  [⚙] [🔒] │
├──────────────────────┬──────────────────────────────────────┤
│ GitHub               │  GitHub                              │
│ octocat              │  Username   octocat         [Copy]   │
│──────────────────────│  Password   ••••••••  [Show][Copy]   │
│ Mail                 │  URL        github.com      [Copy]   │
│ me@example.com       │  Notes      ••••••••        [Show]   │
│                      │                                      │
│                      │                   [Edit]  [Delete]   │
└──────────────────────┴──────────────────────────────────────┘
```

- Search filters as you type, across title, username and URL.
- Passwords are masked until shown, and are masked again when another entry
  is selected.
- The editor has a built-in generator (length and character classes).
- Delete asks for confirmation. Nothing else does.
- Settings: language, auto-lock delay, clipboard delay, change master password.
- English and Turkish. The first run follows the system language; the choice
  can be changed on the first screen or in the settings and is remembered.
  Search is case-insensitive in both, including the Turkish dotted and
  dotless I.
- Dark theme by default, following the legacy plan's one good visual decision.

There is no launcher screen, no secondary windows and no "save" step.

---

## 9. Project layout

```
cmd/pangolin/         entry point
assets/               logo (embedded)
internal/crypto/      KDF, AEAD, key derivation, locked buffers
internal/vault/       header, database, entries, lock state
internal/generator/   password generator
internal/session/     idle auto-lock, clipboard guard
internal/i18n/        interface translations (English, Turkish)
internal/ui/          Fyne screens and theme
```

`internal/` keeps every package private to the application. Only `vault`
imports the database driver, and only `crypto` touches primitives, so either
can be replaced without changing its callers.

---

## 10. Testing

- **crypto**: round trips, tamper detection, wrong key, wrong associated data,
  parameter bounds.
- **vault**: create/unlock/lock, wrong password, header tampering, CRUD,
  ciphertext swapping between entries, password change, persistence across
  reopen, and a scan of the raw files for known plaintext.
- **generator**: length, requested classes present, invalid options rejected.
- **session**: clipboard cleared only when unchanged; idle timer fires and
  resets.
- **i18n**: every string the interface shows has a Turkish translation with
  matching format verbs, and no translation is left unused.
- **ui**: headless walk through create → add → search → copy → lock →
  unlock → edit, language switching, and idle locking.

Tests use reduced Argon2id parameters so the suite runs in seconds.

---

## 11. Known limitations and roadmap

- The Go SQLCipher binding pins SQLCipher 4.4.2. Pangolin only ever runs its
  own fixed SQL against its own file, and stored passwords are additionally
  protected by §5.4, but the binding should be replaced if it falls further
  behind.
- Changing the master password re-wraps MK; it does not rotate it. Someone
  holding an old copy of the header **and** the old password can still open
  the vault. Full key rotation is planned.
- Excluding copied secrets from OS clipboard history needs platform-specific
  code and is planned.
- Planned: encrypted export/import, TOTP, key-file second factor, signed
  release builds.
