// swift-tools-version:6.0
// The iOS app's logic as a local Swift package (ADR 0041): `swift test` runs it without a simulator, so the
// package also builds for macOS. The app target (SwiftUI) comes later from an XcodeGen project.yml.
import PackageDescription

let package = Package(
    name: "AppCore",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [.library(name: "AppCore", targets: ["AppCore"])],
    targets: [
        .target(name: "AppCore"),
        .testTarget(name: "AppCoreTests", dependencies: ["AppCore"]),
    ]
)
