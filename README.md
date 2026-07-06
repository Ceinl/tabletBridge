# tabletBridge

Turn an iPad + Apple Pencil into a graphics-tablet-style input device for a PC.

- **iPad app** — tracks Apple Pencil position and force, streams them over UDP.
- **PC app (Go)** — moves the cursor to follow the pencil, and holds the left
  mouse button down while pencil force exceeds a threshold.

```
 iPad app                         SEND (UDP)            PC app (Go)
 ─────────                        ─────────►            ───────────
 track pencil x/y                 {"x","y",             move cursor to x/y
 track pencil force                "f","p"}             force > X → hold LMB
```

## Protocol

Each pencil sample is one UDP datagram of compact JSON:

```json
{"x":0.5000,"y":0.5000,"f":0.3200,"p":1}
```

| field | meaning                                            |
|-------|----------------------------------------------------|
| `x`   | horizontal position, `0`=left … `1`=right          |
| `y`   | vertical position, `0`=top … `1`=bottom            |
| `f`   | force, `0`=none … `1`=`maximumPossibleForce`        |
| `p`   | phase: `0` began, `1` moved, `2` ended/cancelled   |

Coordinates are normalized so the PC maps them to *its* screen size.

The on-screen keyboard (toggled with the corner button in the iPad app) sends a
different packet — either characters to type, or a Windows virtual-key code:

```json
{"c":"A"}      // type these characters (Unicode, layout-independent)
{"vk":8}       // press a virtual key: Backspace=8, Enter=13
{"btn":"copy"} // fire a deck button by id (PC runs its configured action)
{"get":"deck"} // ask the PC to reply (same UDP socket) with the deck layout
```

## PC app (Go) — Windows

Pure Go: it calls `user32.dll` (`SetCursorPos` / `SendInput`) directly for cursor
and button control, so it needs **no cgo, no gcc, and no external dependencies**.
The receiver is Windows-only.

### Run without cloning

```powershell
go run github.com/Ceinl/tabletBridge/pc@latest
```

### Or build a binary

```powershell
cd pc
go build -o tabletbridge.exe .
.\tabletbridge.exe        # listens on :9000
```

Windows Firewall will prompt on first run — allow it on **Private** networks so
the iPad can reach it.

Flags:

| flag         | default | meaning                                             |
|--------------|---------|-----------------------------------------------------|
| `-port`      | `9000`  | UDP port to listen on                               |
| `-threshold` | `0.15`  | force at/above which the left button is pressed     |
| `-release`   | `0.10`  | force at/below which it is released (hysteresis)    |
| `-smooth`    | `0.0`   | cursor smoothing `0..0.95` (higher = smoother/laggier) |

On start it prints the LAN IPs to point the iPad at.

> Only the mouse-injection layer (`user32.dll`) is Windows-specific. Porting the
> receiver to macOS/Linux means swapping those three calls (`SetCursorPos`,
> `SendInput`, `GetSystemMetrics`) for the platform equivalent.

## iPad app (Swift / SwiftUI)

Source is in `ipad/`. It is plain SwiftUI + `Network.framework` + raw `UITouch`
handling (for force + coalesced touches). Create an Xcode iOS app and add the files:

1. Xcode → **New Project → iOS → App**, name it `TabletBridge`,
   interface **SwiftUI**, language **Swift**.
2. Delete the generated `ContentView.swift`/`App.swift` and drag in everything
   from `ipad/`: `TabletBridgeApp.swift`, `ContentView.swift`,
   `PencilCaptureView.swift`, `UDPSender.swift`.
3. In the target's **Info** tab add **App Transport Security Settings →
   Allow Arbitrary Loads = YES** (local UDP to a raw IP), or a local-networking
   exception. iOS 14+ also shows a **Local Network** permission prompt on first
   send — allow it.
4. Set your development team for signing, pick your iPad, and Run.

> The `ipad/` files will show `No such module 'UIKit'` errors if opened outside
> an iOS target — that's expected; they compile inside the app target.

### Using it

1. Run the PC app; note the `IP:port` it prints.
2. In the iPad app enter that IP + port, tap **Connect** (status → *ready*).
3. Write anywhere on screen with the Apple Pencil. The cursor follows; press
   harder than the threshold to hold the left button (draw/drag).
4. Tap the **corner button** (top-right) to cycle modes: **pencil → keyboard →
   deck**. Keyboard types on the PC (⇧ case, ⌫ backspace, ⏎ enter); deck shows
   your configurable button grid (see Stream Deck mode below).
5. **Two-finger tap** anywhere to hide/show the control panel.

## Stream Deck mode

The iPad's corner button cycles **pencil → keyboard → deck**. The deck is a grid
of configurable buttons; the **PC is the source of truth** for what they do.

- On start, the PC app opens a **web editor** at `http://localhost:<port+1>/`
  (e.g. `http://localhost:9001/`). Add/label buttons, pick an action, and Save —
  it writes `deck.json` next to the app.
- The iPad fetches the layout over the same UDP socket (no extra ports/HTTP) and
  renders the grid. Tapping a button sends its id; the PC runs the action. Use
  the ↻ button on the iPad to re-fetch after editing.

Button action types:

| type     | fields                     | example                                    |
|----------|----------------------------|--------------------------------------------|
| `hotkey` | `mods[]` + `key` (or `vk`) | Ctrl+Shift+M, Alt+F4, F5                    |
| `media`  | `vk`                       | Play/Pause, Next, Mute, Volume ±           |
| `exec`   | `exec` + `args[]`          | launch `obs64.exe`, `notepad`, a script    |

`mods`: `ctrl`, `shift`, `alt`, `win`. `key`: a single character or a name
(`enter`, `tab`, `esc`, `f1`–`f12`, arrows, …). `exec` runs through the shell so
PATH lookups and `.lnk` shortcuts work.

You can also edit `deck.json` by hand; the file is (re)written with a starter set
of buttons on first run. Flags: `-config <path>`, `-webport <n>`, `-noopen`.

## Tuning

- Cursor too jittery → raise `-smooth` (e.g. `0.6`).
- Clicks trigger too early/late → adjust `-threshold` / `-release`.
- Want the whole iPad to map to the whole screen: that's the default (absolute
  mapping, like a drawing tablet).
