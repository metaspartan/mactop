import SwiftUI
import SwiftTerm

struct MactopTerminalView: NSViewRepresentable {
    @EnvironmentObject var settings: AppSettings

    func makeNSView(context: Context) -> LocalProcessTerminalView {
        let terminalView = LocalProcessTerminalView(frame: .zero)
        context.coordinator.terminalView = terminalView
        context.coordinator.launch(with: settings)
        return terminalView
    }

    func updateNSView(_ nsView: LocalProcessTerminalView, context: Context) {
        // Only act when the user explicitly hits "Restart mactop" in
        // Settings — the restartToken change is what triggers relaunch,
        // not every settings edit, so typing in the path field doesn't
        // thrash the process.
        if context.coordinator.lastRestartToken != settings.restartToken {
            context.coordinator.lastRestartToken = settings.restartToken
            context.coordinator.launch(with: settings)
        }
    }

    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    final class Coordinator {
        weak var terminalView: LocalProcessTerminalView?
        var lastRestartToken: Int = 0

        func launch(with settings: AppSettings) {
            guard let terminalView = terminalView else { return }

            // If mactop is already running, terminate it first so we don't
            // end up with two competing processes drawing over each other.
            terminalView.process.terminate()

            guard let mactopPath = settings.resolvedMactopPath() else {
                terminalView.feed(text: "mactop not found.\r\n")
                terminalView.feed(text: "Set a custom path in Settings (Cmd+,), or run `which mactop` in Terminal to locate it.\r\n")
                return
            }

            let args = ["--unit-temp", settings.temperatureUnit.flagValue]

            // Small delay so the previous process fully tears down before
            // we spawn the new one and reset the screen.
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.15) {
                terminalView.getTerminal().resetToInitialState()
                terminalView.startProcess(executable: mactopPath, args: args, environment: nil)
            }
        }
    }
}
