import SwiftUI

@main
struct TabletBridgeApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                // Keep the screen awake during a session.
                .onAppear { UIApplication.shared.isIdleTimerDisabled = true }
                .onDisappear { UIApplication.shared.isIdleTimerDisabled = false }
        }
    }
}
