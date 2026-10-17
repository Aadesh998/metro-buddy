import type { AudioProcessorOptions, Track, TrackProcessor } from 'livekit-client';

// Native WebRTC has no audio processor hook yet; the toggle is hidden until a native module lands.
export const voiceMaskSupported = false;

export function voiceMask(): TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> | undefined {
  return undefined;
}
