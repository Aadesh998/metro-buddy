import { ConnectionState, Room, RoomEvent, type Participant, type RemoteTrack } from 'livekit-client';
import { useCallback, useEffect, useRef, useState } from 'react';

import { post } from '@/api/client';
import type { JoinGrant, StageRole } from '@/api/types';
import { attachAudio, detachAll, startAudio, stopAudio } from './audio';

export type LiveState = {
  status: 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'error';
  role: StageRole | null;
  canSpeak: boolean;
  micOn: boolean;
  speaking: Set<string>;
  error?: string;
};

// The server decides who may publish; the client only follows. When the host stages us,
// LiveKit flips our permission and we switch the mic on; when it is revoked the track dies with it.
export function useLiveRoom(episodeId: string) {
  const room = useRef<Room | null>(null);
  const [state, setState] = useState<LiveState>({ status: 'idle', role: null, canSpeak: false, micOn: false, speaking: new Set() });

  const leave = useCallback(async (notify = true) => {
    const r = room.current;
    room.current = null;
    if (r) await r.disconnect();
    detachAll();
    await stopAudio().catch(() => {});
    if (notify) post(`/v1/episodes/${episodeId}/leave`).catch(() => {});
    setState({ status: 'idle', role: null, canSpeak: false, micOn: false, speaking: new Set() });
  }, [episodeId]);

  const join = useCallback(async () => {
    if (room.current) return;
    setState((s) => ({ ...s, status: 'connecting', error: undefined }));
    try {
      const grant = await post<JoinGrant>(`/v1/episodes/${episodeId}/join`);
      await startAudio();
      const r = new Room({ adaptiveStream: false, dynacast: false });
      room.current = r;
      const syncPerms = () => {
        const can = !!r.localParticipant.permissions?.canPublish;
        setState((s) => ({ ...s, canSpeak: can, micOn: can ? s.micOn : false }));
        if (can) r.localParticipant.setMicrophoneEnabled(true).then(() => setState((s) => ({ ...s, micOn: true }))).catch(() => {});
      };
      r.on(RoomEvent.TrackSubscribed, (track: RemoteTrack) => attachAudio(track))
        .on(RoomEvent.ParticipantPermissionsChanged, (_prev, p: Participant) => {
          if (p.identity === r.localParticipant.identity) syncPerms();
        })
        .on(RoomEvent.ActiveSpeakersChanged, (ps: Participant[]) => setState((s) => ({ ...s, speaking: new Set(ps.map((p) => p.identity)) })))
        .on(RoomEvent.ConnectionStateChanged, (cs: ConnectionState) => {
          if (cs === ConnectionState.Reconnecting) setState((s) => ({ ...s, status: 'reconnecting' }));
          if (cs === ConnectionState.Connected) setState((s) => ({ ...s, status: 'connected' }));
        })
        .on(RoomEvent.Disconnected, () => {
          if (room.current === r) leave(false);
        });
      await r.connect(grant.url, grant.token, { autoSubscribe: true });
      await r.startAudio().catch(() => {});
      setState((s) => ({ ...s, status: 'connected', role: grant.role }));
      syncPerms();
    } catch (e) {
      room.current = null;
      setState((s) => ({ ...s, status: 'error', error: e instanceof Error ? e.message : 'Could not connect' }));
    }
  }, [episodeId, leave]);

  const setMic = useCallback(async (on: boolean) => {
    const r = room.current;
    if (!r) return;
    await r.localParticipant.setMicrophoneEnabled(on);
    setState((s) => ({ ...s, micOn: on }));
  }, []);

  useEffect(() => {
    return () => {
      const r = room.current;
      room.current = null;
      r?.disconnect();
      detachAll();
      stopAudio().catch(() => {});
    };
  }, []);

  return { state, join, leave, setMic };
}
