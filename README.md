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
4. Double-tap with a finger to hide/show the control panel.

## Tuning

- Cursor too jittery → raise `-smooth` (e.g. `0.6`).
- Clicks trigger too early/late → adjust `-threshold` / `-release`.
- Want the whole iPad to map to the whole screen: that's the default (absolute
  mapping, like a drawing tablet).
