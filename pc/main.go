//go:build windows

// Command tabletbridge receives Apple Pencil / on-screen-keyboard packets from
// the iPad app over UDP and drives Windows input: the cursor follows the pencil,
// the left mouse button is held while force exceeds a threshold, and keyboard
// packets are typed into the foreground window.
//
// Pure Go — it calls user32.dll directly, so it needs no cgo/gcc toolchain.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"syscall"
	"unsafe"
)

// --- Win32 bindings (user32.dll) -------------------------------------------

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos     = user32.NewProc("SetCursorPos")
	procSendInput        = user32.NewProc("SendInput")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	smCXScreen = 0
	smCYScreen = 1

	inputMouse    = 0
	inputKeyboard = 1

	mouseEventLeftDown = 0x0002
	mouseEventLeftUp   = 0x0004

	keyEventKeyUp   = 0x0002
	keyEventUnicode = 0x0004
)

// mouseInput mirrors Win32 MOUSEINPUT (32 bytes on amd64).
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input mirrors Win32 INPUT for the mouse case (40 bytes on amd64: the 4-byte
// gap after inputType is the union alignment padding).
type input struct {
	inputType uint32
	mi        mouseInput
}

// keybdInput mirrors Win32 KEYBDINPUT (24 bytes on amd64).
type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// inputKB is the keyboard variant of INPUT, padded to the same 40-byte size as
// the mouse INPUT so SendInput's cbSize is consistent.
type inputKB struct {
	inputType uint32
	ki        keybdInput
	_         [8]byte
}

func screenSize() (int, int) {
	w, _, _ := procGetSystemMetrics.Call(uintptr(smCXScreen))
	h, _, _ := procGetSystemMetrics.Call(uintptr(smCYScreen))
	return int(w), int(h)
}

func setCursor(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
}

func sendMouse(flags uint32) {
	in := input{inputType: inputMouse, mi: mouseInput{dwFlags: flags}}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func pressLeft()   { sendMouse(mouseEventLeftDown) }
func releaseLeft() { sendMouse(mouseEventLeftUp) }

// typeText types a Unicode string via KEYEVENTF_UNICODE (layout-independent).
func typeText(s string) {
	for _, r := range s {
		if r > 0xFFFF {
			continue // non-BMP would need surrogate pairs; skip
		}
		in := inputKB{inputType: inputKeyboard, ki: keybdInput{wScan: uint16(r), dwFlags: keyEventUnicode}}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
		in.ki.dwFlags = keyEventUnicode | keyEventKeyUp
		procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	}
}

// pressVK presses and releases a virtual-key (e.g. Backspace, Enter, arrows).
func pressVK(vk uint16) {
	in := inputKB{inputType: inputKeyboard, ki: keybdInput{wVk: vk}}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	in.ki.dwFlags = keyEventKeyUp
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

// --- packet stream ----------------------------------------------------------

// packet is one sample streamed from the iPad. A pencil sample carries x/y/f/p;
// a keyboard event carries either c (characters to type) or vk (a virtual key).
type packet struct {
	X     float64 `json:"x"`  // 0 = left edge, 1 = right edge of capture area
	Y     float64 `json:"y"`  // 0 = top edge, 1 = bottom edge
	F     float64 `json:"f"`  // normalized force 0..1
	Phase int     `json:"p"`  // 0 began, 1 moved, 2 ended/cancelled
	C     string  `json:"c"`  // characters to type (on-screen keyboard)
	VK    int     `json:"vk"` // virtual-key code for special keys (Backspace, Enter, ...)
}

func main() {
	port := flag.Int("port", 9000, "UDP port to listen on")
	threshold := flag.Float64("threshold", 0.15, "force (0..1) at/above which the left button is held")
	release := flag.Float64("release", 0.10, "force (0..1) at/below which the left button is released (hysteresis)")
	smooth := flag.Float64("smooth", 0.0, "cursor smoothing 0..0.95 (higher = smoother but laggier)")
	flag.Parse()

	if *release > *threshold {
		log.Fatalf("release threshold (%.2f) must be <= press threshold (%.2f)", *release, *threshold)
	}

	addr := net.UDPAddr{Port: *port, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer conn.Close()

	sw, sh := screenSize()
	printBanner(*port, sw, sh, *threshold)

	var (
		buf        = make([]byte, 2048)
		pressed    bool
		haveEMA    bool
		emaX       float64
		emaY       float64
		lastLogged bool
	)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read: %v", err)
			continue
		}

		var p packet
		if err := json.Unmarshal(buf[:n], &p); err != nil {
			continue // ignore malformed packets rather than dropping the stream
		}

		// Keyboard events take priority and are mutually exclusive with pencil.
		if p.C != "" {
			typeText(p.C)
			continue
		}
		if p.VK != 0 {
			pressVK(uint16(p.VK))
			continue
		}

		// Map normalized coords to screen pixels, clamped to the display.
		px := clamp(p.X, 0, 1) * float64(sw-1)
		py := clamp(p.Y, 0, 1) * float64(sh-1)

		// Optional exponential smoothing to tame jitter.
		if *smooth > 0 {
			if !haveEMA {
				emaX, emaY, haveEMA = px, py, true
			} else {
				a := *smooth
				emaX = a*emaX + (1-a)*px
				emaY = a*emaY + (1-a)*py
			}
			px, py = emaX, emaY
		}

		setCursor(int(px+0.5), int(py+0.5))

		// Button logic with hysteresis. On a lift (phase 2) always release.
		lifted := p.Phase == 2
		switch {
		case !pressed && !lifted && p.F >= *threshold:
			pressLeft()
			pressed = true
		case pressed && (lifted || p.F <= *release):
			releaseLeft()
			pressed = false
			haveEMA = false // reset smoothing between strokes
		}

		if pressed != lastLogged {
			if pressed {
				fmt.Printf("\r🖊  DRAW  force=%.2f            ", p.F)
			} else {
				fmt.Printf("\r✋ HOVER force=%.2f            ", p.F)
			}
			lastLogged = pressed
		}
	}
}

func printBanner(port, sw, sh int, threshold float64) {
	ips := localIPs()
	fmt.Println("tabletBridge — PC receiver")
	fmt.Printf("  listening : UDP :%d\n", port)
	fmt.Printf("  screen    : %dx%d px\n", sw, sh)
	fmt.Printf("  threshold : force >= %.2f holds left button\n", threshold)
	if len(ips) > 0 {
		fmt.Println("  point the iPad app at one of these addresses:")
		for _, ip := range ips {
			fmt.Printf("     %s:%d\n", ip, port)
		}
	}
	fmt.Println("  (Ctrl+C to quit)")
}

func localIPs() []string {
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				out = append(out, ip4.String())
			}
		}
	}
	return out
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
