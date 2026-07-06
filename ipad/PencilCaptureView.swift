import SwiftUI
import UIKit

/// SwiftUI wrapper around a UIKit view that captures Apple Pencil touches.
/// We use raw UITouch handling (not PencilKit) because we need per-event
/// location *and* force, including coalesced touches for full fidelity.
struct PencilCaptureView: UIViewRepresentable {
    let onSample: (_ x: Double, _ y: Double, _ force: Double, _ phase: Int) -> Void

    func makeUIView(context: Context) -> TouchView {
        let v = TouchView()
        v.onSample = onSample
        v.backgroundColor = .clear
        v.isMultipleTouchEnabled = true
        return v
    }

    func updateUIView(_ uiView: TouchView, context: Context) {
        uiView.onSample = onSample
    }
}

final class TouchView: UIView {
    var onSample: ((Double, Double, Double, Int) -> Void)?

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        emit(touches, event: event, phase: 0)
    }

    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        emit(touches, event: event, phase: 1)
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        emit(touches, event: event, phase: 2)
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        emit(touches, event: event, phase: 2)
    }

    private func emit(_ touches: Set<UITouch>, event: UIEvent?, phase: Int) {
        guard bounds.width > 0, bounds.height > 0 else { return }
        for touch in touches where touch.type == .pencil {
            // Coalesced touches carry the high-frequency samples the display
            // batched into this single UI event — send them all for smoothness.
            let samples = event?.coalescedTouches(for: touch) ?? [touch]
            for t in samples {
                let p = t.location(in: self)
                let nx = Double(p.x / bounds.width)
                let ny = Double(p.y / bounds.height)
                let maxF = t.maximumPossibleForce
                let force = maxF > 0 ? Double(t.force / maxF) : 0
                onSample?(nx, ny, force, phase)
            }
        }
    }
}
