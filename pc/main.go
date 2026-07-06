// Command tabletbridge receives Apple Pencil packets from the iPad app over UDP
// and drives the local mouse: the cursor follows the pencil, and the left mouse
// button is held down while pencil force exceeds a threshold.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/go-vgo/robotgo"
)

// packet is one sample streamed from the iPad. Coordinates and force are
// normalized to 0..1 so the receiver is independent of the iPad's screen size.
type packet struct {
	X     float64 `json:"x"` // 0 = left edge, 1 = right edge of capture area
	Y     float64 `json:"y"` // 0 = top edge, 1 = bottom edge
	F     float64 `json:"f"` // normalized force 0..1
	Phase int     `json:"p"` // 0 began, 1 moved, 2 ended/cancelled
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

	sw, sh := robotgo.GetScreenSize()
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

		robotgo.Move(int(px+0.5), int(py+0.5))

		// Button logic with hysteresis. On a lift (phase 2) always release.
		lifted := p.Phase == 2
		switch {
		case !pressed && !lifted && p.F >= *threshold:
			robotgo.Toggle("left", "down")
			pressed = true
		case pressed && (lifted || p.F <= *release):
			robotgo.Toggle("left", "up")
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
	if os.Getenv("ROBOTGO_DISABLE") != "" {
		log.Fatal("ROBOTGO_DISABLE set; refusing to control mouse")
	}
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
