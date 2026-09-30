import SwiftUI

@main
struct MacTopApp: App {
    @StateObject private var settings = AppSettings()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(settings)
        }
        .windowResizability(.contentSize)

        // Adds the standard "MacTop > Settings…" menu item and Cmd+,
        Settings {
            SettingsView()
                .environmentObject(settings)
        }
    }
}
