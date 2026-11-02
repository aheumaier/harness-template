import AppCore
import SwiftUI

/// The app's entry point: one screen with the package's greeting. Replace it with the product's first screen.
@main
struct MainApp: App {
    var body: some Scene {
        WindowGroup {
            Text(greeting(for: ""))
        }
    }
}
