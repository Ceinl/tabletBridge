//go:build windows

package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Action is what a deck button does when pressed.
//
//	type "hotkey" : press Mods+Key (or Mods+VK) as a chord      e.g. Ctrl+Shift+M
//	type "media"  : press a single virtual key (media/volume)   e.g. VK 0xB3
//	type "exec"   : launch a program / shell command            e.g. "obs64.exe"
type Action struct {
	Type string   `json:"type"`
	Mods []string `json:"mods,omitempty"` // ctrl, alt, shift, win
	Key  string   `json:"key,omitempty"`  // single char or named key (enter, f5, ...)
	VK   int      `json:"vk,omitempty"`   // explicit virtual-key code
	Exec string   `json:"exec,omitempty"` // program or command to launch
	Args []string `json:"args,omitempty"`
}

// Button is one key on the deck grid.
type Button struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Action Action `json:"action"`
}

// Deck is the full grid configuration, persisted to deck.json.
type Deck struct {
	Cols    int      `json:"cols"`
	Buttons []Button `json:"buttons"`
}

var (
	deckMu   sync.RWMutex
	deck     Deck
	deckPath string
)

func loadDeck(path string) {
	deckPath = path
	data, err := os.ReadFile(path)
	if err != nil {
		// No config yet: start from a sensible default and write it out so the
		// user has something to edit.
		deckMu.Lock()
		deck = defaultDeck()
		deckMu.Unlock()
		if err := saveDeck(); err != nil {
			log.Printf("deck: could not write default config to %s: %v", path, err)
		} else {
			log.Printf("deck: wrote starter config to %s", path)
		}
		return
	}
	var d Deck
	if err := json.Unmarshal(data, &d); err != nil {
		log.Printf("deck: bad JSON in %s: %v (using default)", path, err)
		d = defaultDeck()
	}
	if d.Cols < 1 {
		d.Cols = 4
	}
	deckMu.Lock()
	deck = d
	deckMu.Unlock()
}

func saveDeck() error {
	deckMu.RLock()
	d := deck
	deckMu.RUnlock()
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(deckPath, data, 0o644)
}

func setDeck(d Deck) error {
	if d.Cols < 1 {
		d.Cols = 4
	}
	deckMu.Lock()
	deck = d
	deckMu.Unlock()
	return saveDeck()
}

func getDeck() Deck {
	deckMu.RLock()
	defer deckMu.RUnlock()
	return deck
}

// layoutJSON returns the trimmed layout (ids + labels + cols) the iPad renders.
func layoutJSON() []byte {
	d := getDeck()
	type btn struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	out := struct {
		Cols    int   `json:"cols"`
		Buttons []btn `json:"buttons"`
	}{Cols: d.Cols}
	for _, b := range d.Buttons {
		out.Buttons = append(out.Buttons, btn{b.ID, b.Label})
	}
	data, _ := json.Marshal(out)
	return data
}

func defaultDeck() Deck {
	return Deck{
		Cols: 4,
		Buttons: []Button{
			{ID: "play", Label: "Play/Pause", Action: Action{Type: "media", VK: 0xB3}},
			{ID: "prev", Label: "Prev", Action: Action{Type: "media", VK: 0xB1}},
			{ID: "next", Label: "Next", Action: Action{Type: "media", VK: 0xB0}},
			{ID: "mute", Label: "Mute", Action: Action{Type: "media", VK: 0xAD}},
			{ID: "voldn", Label: "Vol -", Action: Action{Type: "media", VK: 0xAE}},
			{ID: "volup", Label: "Vol +", Action: Action{Type: "media", VK: 0xAF}},
			{ID: "copy", Label: "Copy", Action: Action{Type: "hotkey", Mods: []string{"ctrl"}, Key: "c"}},
			{ID: "paste", Label: "Paste", Action: Action{Type: "hotkey", Mods: []string{"ctrl"}, Key: "v"}},
			{ID: "notepad", Label: "Notepad", Action: Action{Type: "exec", Exec: "notepad"}},
		},
	}
}

// executeButton looks up a button by id and runs its action.
func executeButton(id string) {
	deckMu.RLock()
	var act Action
	found := false
	for _, b := range deck.Buttons {
		if b.ID == id {
			act = b.Action
			found = true
			break
		}
	}
	deckMu.RUnlock()
	if found {
		runAction(act)
	}
}

func runAction(a Action) {
	switch a.Type {
	case "exec":
		launch(a.Exec, a.Args)
	case "media":
		if a.VK != 0 {
			pressVK(uint16(a.VK))
		}
	case "hotkey":
		mods := make([]uint16, 0, len(a.Mods))
		for _, m := range a.Mods {
			if vk, ok := modVK[strings.ToLower(m)]; ok {
				mods = append(mods, vk)
			}
		}
		key := uint16(a.VK)
		if key == 0 {
			key = keyNameToVK(a.Key)
		}
		if key != 0 {
			hotkey(mods, key)
		}
	default:
		if a.VK != 0 {
			pressVK(uint16(a.VK))
		}
	}
}

// launch starts a program/command through the shell so PATH lookups, .lnk
// shortcuts and shell builtins work, and returns immediately (detached).
func launch(cmd string, args []string) {
	if cmd == "" {
		return
	}
	full := append([]string{"/c", "start", "", cmd}, args...)
	if err := exec.Command("cmd", full...).Start(); err != nil {
		log.Printf("deck: exec %q failed: %v", cmd, err)
	}
}

var modVK = map[string]uint16{
	"ctrl": 0x11, "control": 0x11,
	"shift": 0x10,
	"alt":   0x12, "menu": 0x12,
	"win": 0x5B, "super": 0x5B, "cmd": 0x5B, "meta": 0x5B,
}

// keyNameToVK maps a single character or a key name to a Windows virtual-key.
func keyNameToVK(s string) uint16 {
	if s == "" {
		return 0
	}
	if len(s) == 1 {
		c := s[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uint16(c-'a') + 0x41
		case c >= 'A' && c <= 'Z':
			return uint16(c-'A') + 0x41
		case c >= '0' && c <= '9':
			return uint16(c-'0') + 0x30
		}
	}
	switch strings.ToLower(s) {
	case "enter", "return":
		return 0x0D
	case "tab":
		return 0x09
	case "esc", "escape":
		return 0x1B
	case "space":
		return 0x20
	case "backspace":
		return 0x08
	case "delete", "del":
		return 0x2E
	case "insert", "ins":
		return 0x2D
	case "up":
		return 0x26
	case "down":
		return 0x28
	case "left":
		return 0x25
	case "right":
		return 0x27
	case "home":
		return 0x24
	case "end":
		return 0x23
	case "pageup", "pgup":
		return 0x21
	case "pagedown", "pgdn":
		return 0x22
	case "f1":
		return 0x70
	case "f2":
		return 0x71
	case "f3":
		return 0x72
	case "f4":
		return 0x73
	case "f5":
		return 0x74
	case "f6":
		return 0x75
	case "f7":
		return 0x76
	case "f8":
		return 0x77
	case "f9":
		return 0x78
	case "f10":
		return 0x79
	case "f11":
		return 0x7A
	case "f12":
		return 0x7B
	}
	return 0
}
