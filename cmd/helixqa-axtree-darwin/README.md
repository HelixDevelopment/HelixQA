# `helixqa-axtree-darwin`

Swift sidecar exposing the macOS Accessibility (AX) API as JSON over HTTP. Consumed by `pkg/nexus/observe/axtree/darwin.go` on the Go side.

**Status:** operator-action. The Go client is code-complete + 100% tested (M42); this sidecar is the macOS-host-only deliverable an operator needs to build.

## Wire contract

Single GET endpoint:

```
GET /snapshot
```

Response — a single JSON object OR an array of top-level windows:

```json
{
  "role": "AXApplication",
  "title": "HelixQA",
  "description": "",
  "value": "",
  "identifier": "com.example.helixqa",
  "enabled": true,
  "focused": false,
  "selected": false,
  "frame": {"x": 0, "y": 0, "width": 1440, "height": 900},
  "children": [ /* recursive */ ]
}
```

- `role` is the AXRole string (`AXApplication`, `AXWindow`, `AXButton`, ...). The Go client maps 44 known AXRoles to ARIA + falls back to lowercased name-without-prefix for unknowns.
- Array-form top-level wraps into a synthetic `Role=application` root on the Go side.
- Empty bytes, empty array, and scalar JSON all surface as `ErrNoRoot` / parse error.

## Build recipe (reference)

Reference Swift package using `AXUIElement.getAttribute`. Operator must add `NSApplicationCrashOnExceptions = NO` + enable Accessibility entitlement via System Settings.

`Package.swift`:

```swift
// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "helixqa-axtree-darwin",
    platforms: [.macOS(.v13)],
    dependencies: [
        .package(url: "https://github.com/vapor/vapor.git", from: "4.92.0"),
    ],
    targets: [
        .executableTarget(
            name: "axtree",
            dependencies: [
                .product(name: "Vapor", package: "vapor"),
            ]
        ),
    ]
)
```

`Sources/axtree/main.swift` outline:

```swift
import Vapor
import ApplicationServices

let app = try Application(.detect())
defer { app.shutdown() }

app.get("snapshot") { req async throws -> Response in
    let systemWide = AXUIElementCreateSystemWide()
    // Walk systemWide → focused app → children recursively.
    // Convert AXUIElement tree to JSON matching the wire shape.
    let root = try walkAXTree(systemWide)
    let json = try JSONSerialization.data(withJSONObject: root)
    return Response(status: .ok, headers: ["Content-Type": "application/json"], body: .init(data: json))
}

try app.run()
```

## Xcode setup

1. Enable **Accessibility** permission for the built binary: System Settings → Privacy & Security → Accessibility → add `helixqa-axtree-darwin`.
2. (Optional, for unattended runs) sign with a developer certificate: `codesign --force --sign "Developer ID Application: ..." helixqa-axtree-darwin`.
3. Run: `./helixqa-axtree-darwin --hostname 127.0.0.1 --port 17420`.

## Acceptance

1. `GET http://127.0.0.1:17420/snapshot` returns non-empty JSON.
2. `PH6-AXTREE-DARWIN-INT-001` passes.
3. `docs/OPEN_POINTS_CLOSURE.md` §10.3 ScreenCaptureKit + Accessibility entitlement checkboxes ticked.
