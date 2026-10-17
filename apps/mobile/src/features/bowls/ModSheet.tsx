import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Lock, Pin, PinOff, Shield, Trash2, Unlock, VolumeX } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { patch, post as apiPost } from '@/api/client';
import type { Post } from '@/api/types';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';
import { bowlKeys } from './hooks';

type Action = { key: string; label: string; icon: ReactNode; run: () => Promise<unknown>; done: string; danger?: boolean };

export function ModSheet({ bowlId, pinnedId, owner, post, onClose }: { bowlId: string; pinnedId?: string; owner: boolean; post: Post | null; onClose: () => void }) {
  const { t } = useT();
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
      ? { key: 'unpin', label: t('bowls.sheet.unpin'), icon: <PinOff size={18} color={colors.ink} />, run: () => patch(`/v1/bowls/${bowlId}`, { unpin: true }), done: t('bowls.sheet.unpinned') }
      : { key: 'pin', label: t('bowls.sheet.pin'), icon: <Pin size={18} color={colors.ink} />, run: () => patch(`/v1/bowls/${bowlId}`, { pinned_post_id: post.id }), done: t('bowls.sheet.pinned') },
    post.locked
      ? { key: 'unlock', label: t('bowls.sheet.reopen'), icon: <Unlock size={18} color={colors.ink} />, run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'unlock' }), done: t('bowls.sheet.reopened') }
      : { key: 'lock', label: t('bowls.sheet.lock'), icon: <Lock size={18} color={colors.ink} />, run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'lock' }), done: t('bowls.sheet.locked') },
  ];
  if (!post.is_mine && owner) {
    actions.push({
      key: 'make_mod',
      label: t('bowls.sheet.makeMod', { alias: post.author.alias }),
      icon: <Shield size={18} color={colors.ink} />,
      run: () => apiPost(`/v1/bowls/${bowlId}/moderators`, { persona_id: post.author.id }),
      done: t('bowls.sheet.modAdded'),
    });
  }
  if (!post.is_mine) {
    actions.push(
      {
        key: 'mute',
        label: t('bowls.sheet.mute', { alias: post.author.alias }),
        icon: <VolumeX size={18} color={colors.ink} />,
        run: () => apiPost(`/v1/bowls/${bowlId}/mutes`, { persona_id: post.author.id, days: 3 }),
        done: t('bowls.sheet.muted'),
      },
      {
        key: 'remove',
        label: t('bowls.sheet.removePost'),
        icon: <Trash2 size={18} color={colors.danger} />,
        run: () => apiPost(`/v1/posts/${post.id}/mod`, { action: 'remove' }),
        done: t('bowls.sheet.postRemoved'),
        danger: true,
      },
    );
  }
  return (
    <Modal visible transparent animationType="slide" onRequestClose={onClose}>
      <Pressable className="flex-1 bg-black/60" onPress={onClose} accessibilityLabel={t('common.close')} />
      <View className="gap-1 rounded-t-3xl bg-surface px-3 pt-4" style={{ paddingBottom: insets.bottom + 12 }}>
        <Text variant="eyebrow" className="px-3 pb-2">
          {t('bowls.sheet.title')}
        </Text>
        {actions.map((a) => (
          <Pressable key={a.key} disabled={run.isPending} onPress={() => run.mutate(a)} className="flex-row items-center gap-3 rounded-xl px-3 py-3.5 active:bg-raised">
            {a.icon}
            <Text className={a.danger ? 'text-danger' : 'text-ink'}>{a.label}</Text>
          </Pressable>
        ))}
        <Text variant="caption" className="px-3 pt-2">
          {t('bowls.sheet.footer')}
        </Text>
      </View>
    </Modal>
  );
}
