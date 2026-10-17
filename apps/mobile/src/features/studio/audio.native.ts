import { AndroidAudioTypePresets, AudioSession, registerGlobals } from '@livekit/react-native';
import type { RemoteTrack } from 'livekit-client';

registerGlobals();

export async function startAudio() {
  await AudioSession.configureAudio({
    android: { preferredOutputList: ['bluetooth', 'headset', 'speaker'], audioTypeOptions: AndroidAudioTypePresets.communication },
    ios: { defaultOutput: 'speaker' },
  });
  await AudioSession.startAudioSession();
}

export async function stopAudio() {
  await AudioSession.stopAudioSession();
}

// Native WebRTC plays remote audio on its own.
export function attachAudio(_track: RemoteTrack) {}
export function detachAll() {}
