import Foundation
import Combine

enum TemperatureUnit: String, CaseIterable, Identifiable {
    case fahrenheit
    case celsius

    var id: String { rawValue }

    var flagValue: String {
        switch self {
        case .fahrenheit: return "fahrenheit"
        case .celsius: return "celsius"
        }
    }

    var displayName: String {
        switch self {
        case .fahrenheit: return "Fahrenheit"
        case .celsius: return "Celsius"
        }
    }
}

final class AppSettings: ObservableObject {

    @Published var temperatureUnit: TemperatureUnit {
        didSet { UserDefaults.standard.set(temperatureUnit.rawValue, forKey: "temperatureUnit") }
    }

    @Published var customMactopPath: String {
        didSet { UserDefaults.standard.set(customMactopPath, forKey: "customMactopPath") }
    }

    @Published var restartToken: Int = 0

    init() {
        let storedUnit = UserDefaults.standard.string(forKey: "temperatureUnit") ?? TemperatureUnit.fahrenheit.rawValue
        self.temperatureUnit = TemperatureUnit(rawValue: storedUnit) ?? .fahrenheit
        self.customMactopPath = UserDefaults.standard.string(forKey: "customMactopPath") ?? ""
    }

    /// Resolution order:
    /// 1. User's explicit custom path override, if set and it exists
    /// 2. The mactop binary bundled inside this app (Contents/MacOS/mactop)
    /// 3. Standard Homebrew install locations, as a last-resort fallback
    func resolvedMactopPath() -> String? {
        if !customMactopPath.isEmpty, FileManager.default.fileExists(atPath: customMactopPath) {
            return customMactopPath
        }

        if let bundled = Bundle.main.url(forAuxiliaryExecutable: "mactop-binary"),
           FileManager.default.fileExists(atPath: bundled.path) {
            return bundled.path
        }

        let candidatePaths = [
            "/opt/homebrew/bin/mactop",
            "/usr/local/bin/mactop",
            "/usr/bin/mactop"
        ]
        return candidatePaths.first(where: { FileManager.default.fileExists(atPath: $0) })
    }

    func requestRestart() {
        restartToken += 1
    }
}
