import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import type { Message } from '@/api/types';
import { toast } from '@/components/ui/Toast';
import { useMyPersona } from '@/features/me/hooks';
import { t } from '@/i18n';
import { on, onStatus, send as sendFrame, startRealtime, stopRealtime } from '@/lib/realtime';
import { bumpConversation, chatKeys, flushOutbox, patchMessage, upsertMessage } from './hooks';
import { activeConversation, presenceStore, receiptStore, typingStore } from './store';

// Mounted once while signed in: owns the socket and keeps every cache in sync with live events.
export function RealtimeBridge() {
  const qc = useQueryClient();
  const me = useMyPersona().data?.id;
  const meRef = useRef(me);
  useEffect(() => {
    meRef.current = me;
  }, [me]);

  useEffect(() => {
    startRealtime();
    const subs = [
      onStatus((up) => {
        if (!up) return;
        flushOutbox(qc);
        qc.invalidateQueries({ queryKey: chatKeys.conversations });
        const a = activeConversation.get();
        if (a) qc.invalidateQueries({ queryKey: chatKeys.messages(a) });
      }),
      on('message.new', (m: Message) => {
        const mine = m.sender?.id === meRef.current;
        const msg = { ...m, is_mine: mine };
        upsertMessage(qc, msg);
        const viewing = activeConversation.get() === m.conversation_id;
        bumpConversation(qc, msg, mine || viewing ? 0 : 1);
        if (!mine) sendFrame(viewing ? 'read' : 'delivered', { conversation_id: m.conversation_id, message_id: m.id });
        if (m.sender) {
          typingStore.set((s) => {
            const c = { ...(s[m.conversation_id] ?? {}) };
            delete c[m.sender!.id];
            return { ...s, [m.conversation_id]: c };
          });
        }
      }),
      on('message.updated', (d: { id: string; conversation_id: string; body?: string; media?: Message['media'] }) =>
        patchMessage(qc, d.conversation_id, d.id, (x) => (d.media ? { ...x, media: d.media } : { ...x, body: d.body ?? x.body, pii_hidden: true })),
      ),
      on('message.deleted', (d: { id: string; conversation_id: string }) =>
        patchMessage(qc, d.conversation_id, d.id, (x) => ({ ...x, deleted: true, body: '' })),
      ),
      on('typing', (d: { conversation_id: string; persona_id: string }) =>
        typingStore.set((s) => ({ ...s, [d.conversation_id]: { ...(s[d.conversation_id] ?? {}), [d.persona_id]: Date.now() + 4000 } })),
      ),
      on('presence', (d: { persona_id: string; online: boolean }) => presenceStore.set((s) => ({ ...s, [d.persona_id]: d.online }))),
      on('receipt', (d: { conversation_id: string; persona_id: string; message_id: string; kind: 'read' | 'delivered' }) =>
        receiptStore.set((s) => {
          const conv = { ...(s[d.conversation_id] ?? {}) };
          conv[d.persona_id] = { ...(conv[d.persona_id] ?? {}), [d.kind]: d.message_id, ...(d.kind === 'read' ? { delivered: d.message_id } : {}) };
          return { ...s, [d.conversation_id]: conv };
        }),
      ),
      on('dm_request.new', () => {
        qc.invalidateQueries({ queryKey: ['requests'] });
        toast(t('chat.newRequestToast'), 'info');
      }),
      on('conversation.new', () => qc.invalidateQueries({ queryKey: chatKeys.conversations })),
      on('conversation.member_joined', (d: { conversation_id: string }) => qc.invalidateQueries({ queryKey: chatKeys.conversation(d.conversation_id) })),
      on('conversation.member_left', (d: { conversation_id: string }) => qc.invalidateQueries({ queryKey: chatKeys.conversation(d.conversation_id) })),
      on('conversation.member_removed', () => qc.invalidateQueries({ queryKey: chatKeys.conversations })),
      on('room.closed', (d: { conversation_id: string }) => qc.invalidateQueries({ queryKey: chatKeys.conversation(d.conversation_id) })),
      on('notification.new', () => qc.invalidateQueries({ queryKey: chatKeys.inbox })),
      on('match.new', () => qc.invalidateQueries({ queryKey: ['matches'] })),
    ];
    const retry = setInterval(() => flushOutbox(qc), 15_000);
    return () => {
      subs.forEach((u) => u());
      clearInterval(retry);
      stopRealtime();
    };
  }, [qc]);

  return null;
}
