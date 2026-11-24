import SwiftUI

struct ContentView: View {
    @EnvironmentObject var settings: AppSettings

    var body: some View {
        MactopTerminalView()
            .environmentObject(settings)
            .frame(minWidth: 700, minHeight: 420)
    }
}

#Preview {
    ContentView().environmentObject(AppSettings())
}
