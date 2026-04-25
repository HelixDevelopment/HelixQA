# `helixqa-axtree-windows`

Windows sidecar exposing the IUIAutomation COM tree as JSON over HTTP. Consumed by `pkg/nexus/observe/axtree/windows.go` on the Go side.

**Status:** operator-action. The Go client is code-complete + 100% tested on every algorithmic path (M48); this sidecar is the Windows-host deliverable.

## Wire contract

```
GET /snapshot
```

Response — single JSON object OR array (identical to `axtree-darwin`):

```json
{
  "controlType": "Window",
  "name": "HelixQA",
  "automationId": "main-window",
  "className": "HwndWrapper",
  "value": "",
  "helpText": "",
  "isEnabled": true,
  "hasKeyboardFocus": false,
  "isSelected": false,
  "boundingRectangle": [0, 0, 1920, 1080],
  "children": [ /* recursive */ ]
}
```

- `controlType` accepts EITHER a symbolic name (`Button`, `Window`, `Pane`, ...) OR the UIA numeric ID (`50000`, `50032`, ...). The Go client maps 32 symbolic + 19 numeric values to ARIA.
- `boundingRectangle` is `[x, y, width, height]` (NOT `[x1, y1, x2, y2]`).
- Name precedence on the Go side: `name` → `helpText` → `automationId`.

## Build recipe

Two options:

### Option A — Go + go-ole (recommended, matches HelixQA stack)

`main.go`:

```go
// +build windows

package main

import (
    "net/http"
    "encoding/json"
    ole "github.com/go-ole/go-ole"
    "github.com/go-ole/go-ole/oleutil"
)

func main() {
    ole.CoInitialize(0)
    defer ole.CoUninitialize()
    unknown, _ := oleutil.CreateObject("UIAutomationClient.CUIAutomation")
    uia, _ := unknown.QueryInterface(ole.IID_IDispatch)
    http.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
        root := walkUIATree(uia)  // returns a tree of windowsNode structs
        json.NewEncoder(w).Encode(root)
    })
    http.ListenAndServe("127.0.0.1:17421", nil)
}
```

Build:

```powershell
GOOS=windows GOARCH=amd64 go build -o helixqa-axtree-windows.exe ./cmd/helixqa-axtree-windows
```

### Option B — C# / .NET with System.Windows.Automation

Higher-level bindings; shorter code but adds a .NET runtime dependency. Fine for operator convenience but the Go path above is simpler to co-version with HelixQA.

## Deployment

1. Run as the currently-logged-in user (UIA requires desktop session access).
2. For unattended runs: schedule via Task Scheduler with "Run only when user is logged on".
3. Firewall: allow TCP 17421 from the HelixQA orchestrator host.

## Acceptance

1. `GET http://$WIN_HOST:17421/snapshot` returns non-empty JSON from the frontmost app.
2. `PH6-AXTREE-WINDOWS-INT-001` passes.
