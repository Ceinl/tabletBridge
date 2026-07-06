import SwiftUI

struct ContentView: View {
    @StateObject private var sender = UDPSender()
    @AppStorage("pc.host") private var host: String = "192.168.1.10"
    @AppStorage("pc.port") private var portText: String = "9000"

    @State private var lastForce: Double = 0
    @State private var showControls = true
    @State private var keyboardMode = false

    var body: some View {
        ZStack(alignment: .topLeading) {
            Color.black.ignoresSafeArea()

            // Main surface: pencil capture, or the on-screen keyboard.
            if keyboardMode {
                GeometryReader { geo in
                    VStack(spacing: 0) {
                        Spacer(minLength: 0)
                        KeyboardView(
                            onChar: { sender.sendChar($0) },
                            onKey: { sender.sendKey($0) }
                        )
                        .frame(height: geo.size.height * 0.75)
                    }
                }
            } else {
                PencilCaptureView { x, y, force, phase in
                    lastForce = force
                    sender.send(x: x, y: y, force: force, phase: phase)
                }
                .ignoresSafeArea()
            }

            // Corner button: toggle pencil <-> keyboard mode.
            modeToggle
                .frame(maxWidth: .infinity, alignment: .topTrailing)
                .padding()

            if showControls {
                controlPanel
                    .padding()
                    .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 16))
                    .padding()
            }

            // Live force meter (pencil mode only).
            if !keyboardMode {
                VStack {
                    Spacer()
                    forceMeter.padding()
                }
                .frame(maxWidth: .infinity)
            }
        }
        // Two-finger tap toggles the control panel so it stays out of the way.
        .onTapGesture(count: 2) { showControls.toggle() }
    }

    private var modeToggle: some View {
        Button {
            keyboardMode.toggle()
        } label: {
            Image(systemName: keyboardMode ? "pencil.tip.crop.circle" : "keyboard")
                .font(.title2)
                .padding(14)
                .background(.ultraThinMaterial, in: Circle())
        }
        .accessibilityLabel(keyboardMode ? "Switch to pencil" : "Switch to keyboard")
    }

    private var controlPanel: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("tabletBridge").font(.headline)
            HStack {
                TextField("PC IP", text: $host)
                    .textFieldStyle(.roundedBorder)
                    .keyboardType(.decimalPad)
                    .frame(maxWidth: 180)
                TextField("Port", text: $portText)
                    .textFieldStyle(.roundedBorder)
                    .keyboardType(.numberPad)
                    .frame(width: 80)
            }
            HStack {
                Button(action: toggleConnection) {
                    Text(isConnected ? "Disconnect" : "Connect")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                statusLabel
            }
            Text("Pencil draws/moves the cursor. Tap the corner button for a keyboard. Double-tap with a finger to hide this panel.")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: 420)
    }

    private var forceMeter: some View {
        VStack(spacing: 4) {
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule().fill(.gray.opacity(0.3))
                    Capsule().fill(.green)
                        .frame(width: geo.size.width * lastForce)
                }
            }
            .frame(height: 8)
            Text(String(format: "force %.0f%%", lastForce * 100))
                .font(.caption2).foregroundStyle(.secondary)
        }
        .frame(maxWidth: 300)
    }

    private var statusLabel: some View {
        Group {
            switch sender.state {
            case .idle: Label("idle", systemImage: "circle").foregroundStyle(.secondary)
            case .connecting: Label("connecting", systemImage: "arrow.triangle.2.circlepath")
            case .ready: Label("ready", systemImage: "checkmark.circle.fill").foregroundStyle(.green)
            case .failed(let m): Label(m, systemImage: "xmark.octagon.fill").foregroundStyle(.red)
            }
        }
        .font(.caption)
        .lineLimit(1)
    }

    private var isConnected: Bool {
        switch sender.state {
        case .ready, .connecting: return true
        default: return false
        }
    }

    private func toggleConnection() {
        if isConnected {
            sender.disconnect()
        } else if let port = UInt16(portText) {
            sender.connect(host: host, port: port)
        }
    }
}

// MARK: - On-screen QWERTY keyboard

/// A simple QWERTY keyboard. Character keys call `onChar`; special keys
/// (Backspace, Enter) call `onKey` with a Windows virtual-key code.
struct KeyboardView: View {
    let onChar: (String) -> Void
    let onKey: (Int) -> Void

    @State private var shifted = false

    // Windows virtual-key codes.
    private let vkBackspace = 8
    private let vkEnter = 13

    private let rows: [[String]] = [
        ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
        ["q", "w", "e", "r", "t", "y", "u", "i", "o", "p"],
        ["a", "s", "d", "f", "g", "h", "j", "k", "l"],
        ["z", "x", "c", "v", "b", "n", "m"],
        [".", ",", "?", "!", "'", "-"],
    ]

    var body: some View {
        VStack(spacing: 8) {
            keyRow(rows[0])
            keyRow(rows[1])
            keyRow(rows[2])
            HStack(spacing: 8) {
                specialKey(system: shifted ? "shift.fill" : "shift") { shifted.toggle() }
                ForEach(rows[3], id: \.self) { charKey($0) }
                specialKey(system: "delete.left") { onKey(vkBackspace) }
            }
            .frame(maxHeight: .infinity)
            keyRow(rows[4])
            HStack(spacing: 8) {
                Button { onChar(" ") } label: {
                    Text("space").frame(maxWidth: .infinity, maxHeight: .infinity)
                }
                .buttonStyle(KeyCap())
                specialKey(system: "return") { onKey(vkEnter) }
                    .frame(maxWidth: 140)
            }
            .frame(maxHeight: .infinity)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding()
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 16))
        .padding()
    }

    private func keyRow(_ keys: [String]) -> some View {
        HStack(spacing: 8) { ForEach(keys, id: \.self) { charKey($0) } }
            .frame(maxHeight: .infinity)
    }

    private func charKey(_ base: String) -> some View {
        let display = shifted ? base.uppercased() : base
        return Button(display) { onChar(display) }
            .buttonStyle(KeyCap())
    }

    private func specialKey(system: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: system).frame(maxWidth: .infinity)
        }
        .buttonStyle(KeyCap())
    }
}

/// Flat, dark key styling with a press highlight.
struct KeyCap: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.title2.weight(.medium))
            .foregroundStyle(.white)
            .frame(minWidth: 40, minHeight: 52)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(
                (configuration.isPressed ? Color.white.opacity(0.45) : Color.white.opacity(0.18)),
                in: RoundedRectangle(cornerRadius: 8)
            )
    }
}
