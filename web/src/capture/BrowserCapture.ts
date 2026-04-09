/**
 * BrowserCapture - WebRTC screen capture for HelixQA
 * 
 * Uses browser getDisplayMedia() API to capture screen/window/tab
 * and streams via WebRTC to the Go backend server.
 */

export type CaptureSource = 'screen' | 'window' | 'tab';

export interface CaptureOptions {
  source: CaptureSource;
  video?: {
    width?: number;
    height?: number;
    frameRate?: number;
    cursor?: 'always' | 'motion' | 'never';
    displaySurface?: 'monitor' | 'window' | 'browser';
  };
  audio?: boolean;
}

export interface CaptureState {
  isCapturing: boolean;
  isConnected: boolean;
  stream: MediaStream | null;
  videoTrack: MediaStreamTrack | null;
  audioTrack: MediaStreamTrack | null;
  error: string | null;
}

export type CaptureEventType = 
  | 'captureStarted'
  | 'captureStopped'
  | 'streamError'
  | 'connectionStateChange'
  | 'frameCapture'
  | 'trackEnded';

export interface CaptureEvent {
  type: CaptureEventType;
  payload?: unknown;
}

export type CaptureEventHandler = (event: CaptureEvent) => void;

/**
 * BrowserCapture class for screen/window/tab capture via WebRTC
 */
export class BrowserCapture {
  private state: CaptureState = {
    isCapturing: false,
    isConnected: false,
    stream: null,
    videoTrack: null,
    audioTrack: null,
    error: null,
  };

  private eventHandlers: Map<CaptureEventType, CaptureEventHandler[]> = new Map();
  private peerConnection: RTCPeerConnection | null = null;
  private signalingSocket: WebSocket | null = null;
  private signalingUrl: string;
  private roomId: string;
  private clientId: string;
  private iceServers: RTCIceServer[];
  private reconnectAttempts: number = 0;
  private maxReconnectAttempts: number = 3;
  private reconnectDelay: number = 2000;

  constructor(
    signalingUrl: string,
    roomId: string,
    iceServers?: RTCIceServer[]
  ) {
    this.signalingUrl = signalingUrl;
    this.roomId = roomId;
    this.clientId = this.generateClientId();
    this.iceServers = iceServers || [
      { urls: 'stun:stun.l.google.com:19302' },
    ];
  }

  /**
   * Get current capture state
   */
  getState(): CaptureState {
    return { ...this.state };
  }

  /**
   * Check if browser supports getDisplayMedia
   */
  static isSupported(): boolean {
    return !!(navigator.mediaDevices && navigator.mediaDevices.getDisplayMedia);
  }

  /**
   * Request permission and start capture
   */
  async startCapture(options: CaptureOptions): Promise<void> {
    if (!BrowserCapture.isSupported()) {
      throw new Error('Browser does not support getDisplayMedia');
    }

    if (this.state.isCapturing) {
      throw new Error('Capture already in progress');
    }

    try {
      // Build constraints
      const constraints: DisplayMediaStreamOptions = {
        video: {
          width: { ideal: options.video?.width || 1920 },
          height: { ideal: options.video?.height || 1080 },
          frameRate: { ideal: options.video?.frameRate || 30 },
          cursor: options.video?.cursor || 'always',
          displaySurface: options.source,
        } as MediaTrackConstraints,
        audio: options.audio || false,
      };

      // Request display media
      this.state.stream = await navigator.mediaDevices.getDisplayMedia(constraints);
      this.state.videoTrack = this.state.stream.getVideoTracks()[0] || null;
      this.state.audioTrack = this.state.stream.getAudioTracks()[0] || null;

      // Listen for track ended (user stops sharing)
      if (this.state.videoTrack) {
        this.state.videoTrack.onended = () => {
          this.emit('trackEnded', { track: 'video' });
          this.stopCapture();
        };
      }

      this.state.isCapturing = true;
      this.state.error = null;

      this.emit('captureStarted', { stream: this.state.stream });

      // Connect to signaling server
      await this.connectSignaling();

    } catch (err) {
      const error = err instanceof Error ? err.message : 'Unknown error';
      this.state.error = error;
      this.emit('streamError', { error });
      throw err;
    }
  }

  /**
   * Stop capture and cleanup
   */
  stopCapture(): void {
    // Stop all tracks
    if (this.state.stream) {
      this.state.stream.getTracks().forEach(track => track.stop());
      this.state.stream = null;
    }

    this.state.videoTrack = null;
    this.state.audioTrack = null;
    this.state.isCapturing = false;

    // Close peer connection
    if (this.peerConnection) {
      this.peerConnection.close();
      this.peerConnection = null;
    }

    // Close signaling socket
    if (this.signalingSocket) {
      this.signalingSocket.close();
      this.signalingSocket = null;
    }

    this.state.isConnected = false;
    this.emit('captureStopped', {});
  }

  /**
   * Connect to WebRTC signaling server
   */
  private async connectSignaling(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.signalingUrl);

      ws.onopen = () => {
        console.log('[BrowserCapture] Signaling connected');
        this.signalingSocket = ws;
        this.reconnectAttempts = 0;
        
        // Join room
        this.sendSignalingMessage({
          type: 'join',
          roomId: this.roomId,
          clientId: this.clientId,
        });

        // Initialize WebRTC
        this.initializeWebRTC()
          .then(resolve)
          .catch(reject);
      };

      ws.onmessage = (event) => {
        this.handleSignalingMessage(JSON.parse(event.data));
      };

      ws.onclose = () => {
        console.log('[BrowserCapture] Signaling disconnected');
        this.state.isConnected = false;
        this.emit('connectionStateChange', { state: 'disconnected' });
        
        // Attempt reconnect
        if (this.state.isCapturing && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          setTimeout(() => {
            console.log(`[BrowserCapture] Reconnecting (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
            this.connectSignaling();
          }, this.reconnectDelay);
        }
      };

      ws.onerror = (error) => {
        console.error('[BrowserCapture] Signaling error:', error);
        reject(error);
      };
    });
  }

  /**
   * Initialize WebRTC peer connection
   */
  private async initializeWebRTC(): Promise<void> {
    const config: RTCConfiguration = {
      iceServers: this.iceServers,
    };

    this.peerConnection = new RTCPeerConnection(config);

    // Add stream tracks
    if (this.state.stream) {
      this.state.stream.getTracks().forEach(track => {
        if (this.peerConnection && this.state.stream) {
          this.peerConnection.addTrack(track, this.state.stream);
        }
      });
    }

    // Handle ICE candidates
    this.peerConnection.onicecandidate = (event) => {
      if (event.candidate) {
        this.sendSignalingMessage({
          type: 'ice',
          roomId: this.roomId,
          clientId: this.clientId,
          ice: event.candidate.toJSON(),
        });
      }
    };

    // Handle connection state changes
    this.peerConnection.onconnectionstatechange = () => {
      const state = this.peerConnection?.connectionState;
      console.log('[BrowserCapture] Connection state:', state);
      
      this.state.isConnected = state === 'connected';
      this.emit('connectionStateChange', { state });

      if (state === 'failed' || state === 'closed') {
        this.emit('streamError', { error: `Connection ${state}` });
      }
    };

    // Create and send offer
    const offer = await this.peerConnection.createOffer();
    await this.peerConnection.setLocalDescription(offer);

    this.sendSignalingMessage({
      type: 'offer',
      roomId: this.roomId,
      clientId: this.clientId,
      sdp: offer,
    });
  }

  /**
   * Handle incoming signaling messages
   */
  private handleSignalingMessage(msg: SignalingMessage): void {
    console.log('[BrowserCapture] Received:', msg.type);

    switch (msg.type) {
      case 'answer':
        this.handleAnswer(msg);
        break;
      case 'ice':
        this.handleICECandidate(msg);
        break;
      case 'error':
        console.error('[BrowserCapture] Signaling error:', msg.error);
        this.state.error = msg.error || 'Unknown error';
        this.emit('streamError', { error: msg.error });
        break;
    }
  }

  /**
   * Handle SDP answer from server
   */
  private async handleAnswer(msg: SignalingMessage): Promise<void> {
    if (!this.peerConnection || !msg.sdp) return;

    await this.peerConnection.setRemoteDescription(
      new RTCSessionDescription(msg.sdp)
    );
  }

  /**
   * Handle ICE candidate from server
   */
  private async handleICECandidate(msg: SignalingMessage): Promise<void> {
    if (!this.peerConnection || !msg.ice) return;

    await this.peerConnection.addIceCandidate(
      new RTCIceCandidate(msg.ice)
    );
  }

  /**
   * Send signaling message
   */
  private sendSignalingMessage(msg: SignalingMessage): void {
    if (this.signalingSocket?.readyState === WebSocket.OPEN) {
      this.signalingSocket.send(JSON.stringify(msg));
    }
  }

  /**
   * Capture a single frame as ImageData
   */
  async captureFrame(): Promise<ImageData> {
    if (!this.state.videoTrack) {
      throw new Error('No video track available');
    }

    const video = document.createElement('video');
    video.srcObject = new MediaStream([this.state.videoTrack]);
    
    return new Promise((resolve, reject) => {
      video.onloadedmetadata = () => {
        video.play();
        
        const canvas = document.createElement('canvas');
        canvas.width = video.videoWidth;
        canvas.height = video.videoHeight;
        
        const ctx = canvas.getContext('2d');
        if (!ctx) {
          reject(new Error('Failed to get canvas context'));
          return;
        }
        
        ctx.drawImage(video, 0, 0);
        const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
        
        video.pause();
        video.srcObject = null;
        
        resolve(imageData);
      };
      
      video.onerror = (err) => {
        reject(err);
      };
    });
  }

  /**
   * Get frame as ArrayBuffer (for binary transport)
   */
  async captureFrameBuffer(format: 'png' | 'jpeg' = 'png'): Promise<ArrayBuffer> {
    const imageData = await this.captureFrame();
    
    const canvas = document.createElement('canvas');
    canvas.width = imageData.width;
    canvas.height = imageData.height;
    
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Failed to get canvas context');
    }
    
    ctx.putImageData(imageData, 0, 0);
    
    const mimeType = format === 'jpeg' ? 'image/jpeg' : 'image/png';
    const blob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob((b) => {
        if (b) resolve(b);
        else reject(new Error('Failed to create blob'));
      }, mimeType, 0.95);
    });
    
    return await blob.arrayBuffer();
  }

  /**
   * Event subscription
   */
  on(event: CaptureEventType, handler: CaptureEventHandler): () => void {
    if (!this.eventHandlers.has(event)) {
      this.eventHandlers.set(event, []);
    }
    this.eventHandlers.get(event)!.push(handler);

    // Return unsubscribe function
    return () => {
      const handlers = this.eventHandlers.get(event);
      if (handlers) {
        const index = handlers.indexOf(handler);
        if (index > -1) {
          handlers.splice(index, 1);
        }
      }
    };
  }

  /**
   * Emit event to subscribers
   */
  private emit(type: CaptureEventType, payload: unknown): void {
    const handlers = this.eventHandlers.get(type);
    if (handlers) {
      handlers.forEach(handler => {
        try {
          handler({ type, payload });
        } catch (err) {
          console.error('[BrowserCapture] Event handler error:', err);
        }
      });
    }
  }

  /**
   * Generate unique client ID
   */
  private generateClientId(): string {
    return `browser_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }
}

/**
 * Signaling message interface
 */
interface SignalingMessage {
  type: string;
  roomId: string;
  clientId: string;
  sdp?: RTCSessionDescriptionInit;
  ice?: RTCIceCandidateInit;
  error?: string;
  timestamp?: number;
}

export default BrowserCapture;
