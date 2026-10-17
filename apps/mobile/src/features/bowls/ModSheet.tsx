import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Lock, Pin, PinOff, Shield, Trash2, Unlock, VolumeX } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { patch, post as apiPost } from '@/api/client';
import type { Post } from '@/api/types';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { colors } from '@/lib/theme';
import { bowlKeys } from './hooks';

type Action = { key: string; label: string; icon: ReactNode; run: () => Promise<unknown>; done: string; danger?: boolean };

export function ModSheet({ bowlId, pinnedId, owner, post, onClose }: { bowlId: string; pinnedId?: string; owner: boolean; post: Post | null; onClose: () => void }) {
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const run = useMutation({
    mutationFn: (a: Action) => a.run(),
    onSuccess: (_r, a) => {
      toast(a.done, 'success');
      qc.invalidateQueries({ queryKey: ['feed'] });
      qc.invalidateQueries({ queryKey: bowlKeys.bowl(bowlId) });
      if (post) qc.invalidateQueries({ queryKey: bowlKeys.post(post.id) });
      onClose();
    },
    onError: (e) => toast(e.message),
  });
  if (!post) return null;
  const pinned = pinnedId === post.id;
  const actions: Action[] = [
    pinned
      ? { key: 'unpin', label: 'Unpin', icon: <PinOff size={18} color={colors.ink} />, run: () => patch(`/v1/bowls/${bowlId}`, { unpin: true }), done: 'Unpinned' }
      : { key: 'pin', label: 'Pin to top of bowl', icon: <Pin size={18} color={colors.ink} />, run: () => patch(`/v1/bowls/${bowlId}`, { pinned_post_id: post.id }), done: 'Pinned' },
    post.locked
      ? { key: 'unlock', label: 'Reopen comments', icon: <Unlock size={18} color={colors.ink} />, run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'unlock' }), done: 'Comments reopened' }
      : { key: 'lock', label: 'Close comments', icon: <Lock size={18} color={colors.ink} />, run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'lock' }), done: 'Comments closed' },
  ];
  if (!post.is_mine && owner) {
    actions.push({
      key: 'make_mod',
      label: `Make ${post.author.alias} a moderator`,
      icon: <Shield size={18} color={colors.ink} />,
      run: () => apiPost(`/v1/bowls/${bowlId}/moderators`, { persona_id: post.author.id }),
      done: 'Moderator added',
    });
  }
  if (!post.is_mine) {
    actions.push(
      {
        key: 'mute',
        label: `Mute ${post.author.alias} for 3 days`,
        icon: <VolumeX size={18} color={colors.ink} />,
        run: () => apiPost(`/v1/bowls/${bowlId}/mutes`, { persona_id: post.author.id, days: 3 }),
        done: 'Muted for 3 days',
      },
      {
        key: 'remove',
        label: 'Remove post',
        icon: <Trash2 size={18} color={colors.danger} />,
        run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'remove' }),
        done: 'Post removed',
        danger: true,
      },
    );
  }
  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <Pressable className="flex-1 bg-black/60" onPress={onClose} accessibilityLabel="Close" />
      <View className="gap-1 rounded-t-3xl bg-surface px-3 pt-4" style={{ paddingBottom: insets.bottom + 12 }}>
        <Text variant="caption" className="px-3 pb-2 uppercase tracking-wider">
          Moderator tools
        </Text>
        {actions.map((a) => (
          <Pressable key={a.key} disabled={run.isPending} onPress={() => run.mutate(a)} className="flex-row items-center gap-3 rounded-xl px-3 py-3.5 active:bg-raised">
            {a.icon}
            <Text className={a.danger ? 'text-danger' : 'text-ink'}>{a.label}</Text>
          </Pressable>
        ))}
        <Text variant="caption" className="px-3 pt-2">
          Your actions are visible to the other moderators in the mod log.
        </Text>
      </View>
    </Modal>
  );
}
