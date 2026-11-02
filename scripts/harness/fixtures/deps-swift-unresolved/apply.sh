#!/usr/bin/env bash
# Runs in the repo copy: adds a Swift package that depends on a remote package and commits no Package.resolved.
set -euo pipefail
mkdir -p swift-remote
cat >swift-remote/Package.swift <<'SWIFT'
// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Remote",
    dependencies: [.package(url: "https://github.com/apple/swift-argument-parser", from: "1.0.0")]
)
SWIFT
