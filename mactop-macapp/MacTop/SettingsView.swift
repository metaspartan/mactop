import SwiftUI

struct SettingsView: View {
    var body: some View {
        TabView {
            GeneralSettingsView()
                .tabItem { Label("General", systemImage: "gearshape") }

            AboutView()
                .tabItem { Label("About", systemImage: "info.circle") }
        }
    }
}

struct GeneralSettingsView: View {
    @EnvironmentObject var settings: AppSettings

    var body: some View {
        Form {
            Section {
                Picker("Temperature unit", selection: $settings.temperatureUnit) {
                    ForEach(TemperatureUnit.allCases) { unit in
                        Text(unit.displayName).tag(unit)
                    }
                }
                .pickerStyle(.radioGroup)
            }

            Section {
                TextField("Custom mactop path (optional)", text: $settings.customMactopPath)
                    .textFieldStyle(.roundedBorder)
                Text("Leave blank to use the bundled mactop, or auto-detect from Homebrew.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Section {
                Button("Restart mactop with these settings") {
                    settings.requestRestart()
                }
            }
        }
        .padding(20)
        .frame(width: 420)
    }
}

#Preview {
    SettingsView().environmentObject(AppSettings())
}
