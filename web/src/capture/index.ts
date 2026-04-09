/**
 * HelixQA Web Capture Module
 * 
 * Provides browser-based screen capture using WebRTC.
 * Supports screen, window, and tab capture across Chrome, Firefox, and Edge.
 */

export { BrowserCapture } from './BrowserCapture';
export type {
  CaptureSource,
  CaptureOptions,
  CaptureState,
  CaptureEventType,
  CaptureEvent,
  CaptureEventHandler,
} from './BrowserCapture';

// Convenience function for quick capture
export async function captureScreen(
  signalingUrl: string,
  roomId: string
): Promise<BrowserCapture> {
  const capture = new BrowserCapture(signalingUrl, roomId);
  await capture.startCapture({ source: 'screen' });
  return capture;
}

// Check browser support
export function isCaptureSupported(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    !!(navigator.mediaDevices && navigator.mediaDevices.getDisplayMedia)
  );
}

// Get supported constraints
export function getSupportedConstraints(): MediaTrackSupportedConstraints | null {
  if (typeof navigator === 'undefined' || !navigator.mediaDevices) {
    return null;
  }
  return navigator.mediaDevices.getSupportedConstraints();
}
