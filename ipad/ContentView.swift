import SwiftUI

struct ContentView: View {
    @StateObject private var sender = UDPSender()
    @AppStorage("pc.host") private var host: String = "192.168.1.10"
    @AppStorage("pc.port") private var portText: String = "9000"

    @State private var lastForce: Double = 0
    @State private var showControls = true

    var body: some View {
        ZStack(alignment: .top) {
            Color.black.ignoresSafeArea()

            // Full-screen capture surface.
            PencilCaptureView { x, y, force, phase in
                lastForce = force
                sender.send(x: x, y: y, force: force, phase: phase)
            }
            .ignoresSafeArea()

            if showControls {
                controlPanel
                    .padding()
                    .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: 16))
                    .padding()
            }

            // Live force meter at the bottom.
            VStack {
                Spacer()
                forceMeter
                    .padding()
            }
        }
        // Two-finger tap to hide/show the control panel so it doesn't get in the way.
        .onTapGesture(count: 2) { showControls.toggle() }
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
            Text("Write with the Apple Pencil anywhere. Double-tap with a finger to hide this panel.")
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
