import Foundation
import Network

/// Fire-and-forget UDP sender used to stream pencil samples to the PC.
/// UDP is chosen for low latency: a dropped sample just means one skipped frame.
final class UDPSender: ObservableObject {
    enum State: Equatable { case idle, connecting, ready, failed(String) }

    @Published private(set) var state: State = .idle

    private var connection: NWConnection?
    private let queue = DispatchQueue(label: "udp.sender")

    func connect(host: String, port: UInt16) {
        disconnect()
        guard let nwPort = NWEndpoint.Port(rawValue: port) else {
            state = .failed("bad port")
            return
        }
        state = .connecting
        let conn = NWConnection(host: NWEndpoint.Host(host), port: nwPort, using: .udp)
        conn.stateUpdateHandler = { [weak self] newState in
            DispatchQueue.main.async {
                switch newState {
                case .ready: self?.state = .ready
                case .failed(let e): self?.state = .failed(e.localizedDescription)
                case .waiting(let e): self?.state = .failed(e.localizedDescription)
                default: break
                }
            }
        }
        connection = conn
        conn.start(queue: queue)
    }

    func disconnect() {
        connection?.cancel()
        connection = nil
        state = .idle
    }

    /// Send one sample. `x`,`y`,`force` are normalized to 0...1.
    /// phase: 0 began, 1 moved, 2 ended.
    func send(x: Double, y: Double, force: Double, phase: Int) {
        guard let conn = connection, state == .ready else { return }
        // Compact JSON keeps packets small at high sample rates.
        let json = String(
            format: "{\"x\":%.4f,\"y\":%.4f,\"f\":%.4f,\"p\":%d}",
            x, y, force, phase
        )
        conn.send(content: json.data(using: .utf8), completion: .idempotent)
    }
}
