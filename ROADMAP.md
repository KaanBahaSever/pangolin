# Roadmap

Ideas for Pangolin after 1.3, grouped by how much work they are. Nothing here
is promised; the order inside each group is a rough suggestion of value for
effort. Anything that touches storage or cryptography gets a design note in
[architecture.md](architecture.md) before code.

The rules that every item must keep:

- Nothing leaves the computer unless the user asks for it, each time.
- No new cryptography; only well-reviewed primitives and libraries.
- Old vaults keep opening (see `internal/vault/testdata`).

## Easy (an afternoon or two)

| Idea | Why |
|---|---|
| Keyboard shortcuts: Ctrl+F search, Ctrl+N new entry, Ctrl+L lock, Ctrl+Shift+C copy password, Esc cancel | Faster daily use without the mouse |
| Remember window size and position | Small comfort, stored with the other preferences |
| "Open website" button next to the URL | One click from entry to login page |
| Sort the list by name or by last change | Large vaults become easier to scan |
| Relative dates ("changed 3 months ago") | Easier to spot passwords worth rotating |
| Ask before leaving the editor with unsaved changes | Prevents losing an edit by clicking another entry |
| Duplicate an entry | Handy for several accounts on one site |
| Lock when the window is minimised (option) | Extra caution on shared machines |
| Dependabot for Go modules and the pinned GitHub Actions | Security updates arrive as pull requests |
| Run govulncheck in CI | Catches vulnerable dependencies before a release |
| Fuzz the header parser and field decryption (`go test -fuzz`) | Cheap insurance for code that reads untrusted files |

## Medium (a week or so each)

| Idea | Why |
|---|---|
| Password strength meter (zxcvbn) for the master password and in the editor | Length alone is a weak signal |
| Encrypted export and import of a Pangolin backup file | Moving to a new computer without copying folders by hand |
| Import from Bitwarden, KeePass and browser CSV exports | Easier to switch to Pangolin |
| Automatic encrypted backups: keep the last few copies of the vault | Protection against accidental deletion and bad syncs |
| TOTP codes (two-factor) with a countdown | A common reason people keep a second app |
| Favourites and tags (schema v2, with a migration and a new test fixture) | Organising a growing vault |
| Custom fields per entry (PIN, security answers, licence keys) | Not everything is a username and password |
| Keep copied passwords out of clipboard history (Windows `ExcludeClipboardContentFromMonitorProcessing`, macOS concealed pasteboard type) | Closes the gap noted in architecture.md §7.4 |
| Lock when the operating system locks or goes to sleep | Matches what users expect from a password manager |
| Password health report: reused, weak and old passwords, computed locally | Turns the vault into a to-do list |
| Rotate the master key when the master password changes | Old header copies stop working with the old password (§11) |
| Calibrate Argon2id on vault creation (target about half a second) | Stronger on fast machines, still quick on slow ones |
| Windows installer with a Start menu entry; winget, Scoop and Homebrew packages | Installation and updates the way users expect |
| Signed checksums (minisign or cosign) and GitHub build attestations | Users can verify that a download was built from this repository |
| Opt-in breach check through Have I Been Pwned's k-anonymity API | Finds leaked passwords without sending them anywhere |

## Advanced (larger projects)

| Idea | Why |
|---|---|
| Code signing: Apple Developer ID with notarisation, Windows Authenticode | Removes the SmartScreen and Gatekeeper warnings |
| Second factor for unlocking: key file, or a hardware key (YubiKey challenge–response, FIDO2 `hmac-secret`) | A stolen vault plus a guessed password is no longer enough |
| Serverless sync: merge two copies of a vault entry by entry, for use with Syncthing or a cloud folder | Several computers without a Pangolin server |
| Replace or maintain the SQLCipher binding (currently pinned to SQLCipher 4.4.2) | The weakest dependency today |
| A password field that never turns text into Go strings | Shrinks the memory residue described in §7.4 |
| Share an entry with someone by encrypting it to their public key (age) | Family and team use without a server |
| Browser extension with native messaging for autofill | Big convenience, but a large new attack surface; needs its own threat model |
| Read-only mobile companion | Access on the go, starting with the least risky mode |
| Passkey storage | Where accounts are heading |
| Reproducible builds and an independent security audit | The strongest answer to "can I trust it?" |
