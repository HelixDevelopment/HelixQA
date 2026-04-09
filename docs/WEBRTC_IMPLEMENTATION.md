# WebRTC Implementation for HelixQA

## Overview

This document describes the WebRTC implementation for browser-based screen capture in the HelixQA video processing pipeline. It enables real-time screen sharing from web browsers (Chrome, Firefox, Edge) to the Go backend for video analysis.

## Architecture

```
┌─────────────────┐      WebSocket       ┌──────────────────┐
│   Web Browser   │ ◄──────────────────► │   Go Backend     │
│  (TypeScript)   │     Signaling        │  (Pion WebRTC)   │
└────────┬────────┘                      └────────┬─────────┘
         │                                        │
         │ WebRTC Peer Connection                 │
         │ (STUN/TURN/ICE)                        │
         ▼                                        ▼
┌─────────────────┐                      ┌──────────────────┐
│  getDisplayMedia│                      │  Video Track     │
│  Screen Capture │────────────────────►│  Processing      │
└─────────────────┘    RTP Streams       └──────────────────┘
```

## Components

### 1. Go Backend (`pkg/streaming/`)

#### `webrtc_server.go`
- **WebRTCServer**: Main server managing peer connections
- **Room**: Manages clients in a session
- **Client**: Represents a connected browser client
- **SignalingMessage**: WebSocket message protocol

Key features:
- WebSocket-based signaling
- ICE candidate exchange
- SDP offer/answer handling
- Room-based session management
- Connection state monitoring

#### `webrtc_handler.go`
- **WebRTCHandler**: HTTP handlers for WebRTC endpoints
- Prometheus metrics integration
- Configuration endpoint
- Statistics endpoints

### 2. Browser Client (`web/src/capture/`)

#### `BrowserCapture.ts`
- **BrowserCapture**: Main capture class
- Uses `getDisplayMedia()` API
- WebRTC peer connection management
- Automatic reconnection
- Frame extraction

#### `index.ts`
- Module exports
- Convenience functions
- Browser support detection

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws/webrtc` | WebSocket | Signaling connection |
| `/api/webrtc/config` | GET | ICE servers configuration |
| `/api/webrtc/stats` | GET | Server statistics |
| `/api/webrtc/rooms/{id}` | GET | Room statistics |

## Signaling Protocol

### Message Types

```typescript
interface SignalingMessage {
  type: 'join' | 'leave' | 'offer' | 'answer' | 'ice' | 'error';
  roomId: string;
  clientId: string;
  sdp?: RTCSessionDescription;
  ice?: RTCIceCandidate;
  error?: string;
}
```

### Connection Flow

1. **Join Room**
   ```json
   { "type": "join", "roomId": "test-room" }
   ```

2. **Send Offer**
   ```json
   { "type": "offer", "roomId": "test-room", "sdp": {...} }
   ```

3. **Receive Answer**
   ```json
   { "type": "answer", "roomId": "test-room", "sdp": {...} }
   ```

4. **Exchange ICE**
   ```json
   { "type": "ice", "roomId": "test-room", "ice": {...} }
   ```

## Usage

### Go Backend

```go
package main

import (
    "net/http"
    "digital.vasic.helixqa/pkg/streaming"
)

func main() {
    // Create WebRTC server
    config := streaming.DefaultWebRTCConfig()
    server := streaming.NewWebRTCServer(config)
    
    // Create handler and register routes
    handler := streaming.NewWebRTCHandler(server)
    mux := http.NewServeMux()
    handler.RegisterRoutes(mux)
    
    // Start server
    http.ListenAndServe(":8080", mux)
}
```

### Browser Client

```typescript
import { BrowserCapture } from './capture';

const capture = new BrowserCapture(
  'ws://localhost:8080/ws/webrtc',
  'test-room'
);

// Start capture
await capture.startCapture({
  source: 'screen',
  video: {
    width: 1920,
    height: 1080,
    frameRate: 30
  }
});

// Capture a frame
const imageData = await capture.captureFrame();

// Stop capture
capture.stopCapture();
```

## Configuration

### ICE Servers

Default configuration uses Google's public STUN server:

```go
config := &WebRTCConfig{
    ICEServers: []webrtc.ICEServer{
        {URLs: []string{"stun:stun.l.google.com:19302"}},
    },
}
```

For production, configure TURN servers:

```go
config := &WebRTCConfig{
    ICEServers: []webrtc.ICEServer{
        {URLs: []string{"stun:stun.example.com:3478"}},
        {
            URLs:       []string{"turn:turn.example.com:3478"},
            Username:   "user",
            Credential: "password",
        },
    },
}
```

## Browser Support

| Browser | Screen Capture | Window Capture | Tab Capture |
|---------|---------------|----------------|-------------|
| Chrome 72+ | ✅ | ✅ | ✅ |
| Firefox 66+ | ✅ | ✅ | ❌ |
| Edge 79+ | ✅ | ✅ | ✅ |
| Safari 13+ | ✅ | ❌ | ❌ |

## Metrics

Prometheus metrics exposed:

- `helixqa_webrtc_connections_total` - Total connections
- `helixqa_webrtc_connections_active` - Active connections
- `helixqa_webrtc_signaling_messages_total` - Signaling messages by type

## Testing

Run WebRTC tests:

```bash
cd HelixQA
go test -v ./pkg/streaming/...
```

Run all tests:

```bash
go test -v ./...
```

## Security Considerations

1. **CORS**: Configure allowed origins in production
2. **TURN Authentication**: Use secure credentials
3. **Room Access**: Implement authentication/authorization
4. **Rate Limiting**: Add rate limits for WebSocket connections

## Troubleshooting

### Connection Fails
- Check STUN/TURN server availability
- Verify firewall rules for UDP ports
- Check browser console for errors

### No Video
- Ensure `getDisplayMedia` permission granted
- Check video track constraints
- Verify codec support

### High Latency
- Use closer STUN/TURN servers
- Check network conditions
- Reduce video resolution

## Dependencies

### Go
- `github.com/pion/webrtc/v4` - WebRTC implementation
- `github.com/gorilla/websocket` - WebSocket server

### TypeScript
- Native browser APIs (no dependencies)

## License

Part of HelixQA - MIT License
