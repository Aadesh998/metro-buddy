import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Image } from 'expo-image';
import { router } from 'expo-router';
import { Check, Eye } from 'lucide-react-native';
import { View } from 'react-native';

import { get, post } from '@/api/client';
import type { RevealLevel, RevealState } from '@/api/types';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useT, type Key } from '@/i18n';
import { confirm } from '@/lib/confirm';
import { colors } from '@/lib/theme';

const STEPS: { level: RevealLevel; label: Key; lower: Key }[] = [
  { level: 'first_name', label: 'chat.reveal.first_name', lower: 'chat.reveal.first_nameLower' },
  { level: 'photo', label: 'chat.reveal.photo', lower: 'chat.reveal.photoLower' },
  { level: 'contact', label: 'chat.reveal.contact', lower: 'chat.reveal.contactLower' },
];
const order: Record<RevealLevel, number> = { none: 0, first_name: 1, photo: 2, contact: 3 };

export function RevealPanel({ conversationId, otherAlias }: { conversationId: string; otherAlias: string }) {
  const { t } = useT();
  const qc = useQueryClient();
  const key = ['reveal', conversationId];
  const q = useQuery({ queryKey: key, queryFn: () => get<RevealState>(`/v1/conversations/${conversationId}/reveal`) });
  const step = useMutation({
    mutationFn: (level: RevealLevel) => post<RevealState>(`/v1/conversations/${conversationId}/reveal`, { level }),
    onSuccess: (d) => qc.setQueryData(key, d),
    onError: (e) => toast(e.message),
  });
  const r = q.data;
  if (!r) return null;
  const next = STEPS.find((s) => order[s.level] === order[r.mine] + 1);
  const canOfferNext = next && r.can_offer.includes(next.level);
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center gap-2">
        <Eye size={18} color={colors.brand} />
        <Text variant="label">{t('chat.reveal.title')}</Text>
      </View>
      <View className="flex-row gap-2">
        {STEPS.map((s) => {
          const mutual = order[r.mutual] >= order[s.level];
          const mine = order[r.mine] >= order[s.level];
          const theirs = order[r.theirs] >= order[s.level];
          return (
            <View key={s.level} className={`flex-1 items-center gap-1 rounded-xl p-2 ${mutual ? 'bg-mint/15' : 'bg-raised'}`}>
              <Text className="text-xs font-semibold">{t(s.label)}</Text>
              <Text className="text-[10px] text-muted">{mutual ? t('chat.reveal.shared') : mine ? t('chat.reveal.waiting') : theirs ? t('chat.reveal.theyReady') : '—'}</Text>
            </View>
          );
        })}
      </View>
      {r.mutual !== 'none' ? (
        <View className="flex-row items-center gap-3 rounded-xl bg-raised p-3">
          {r.revealed.photo_url ? <Image source={{ uri: r.revealed.photo_url }} style={{ width: 48, height: 48, borderRadius: 24 }} /> : null}
          <View className="flex-1">
            <Text variant="label">{r.revealed.first_name}</Text>
            {r.revealed.contact ? <Text selectable className="text-sm text-brand">{r.revealed.contact}</Text> : null}
          </View>
          <Check size={16} color={colors.mint} />
        </View>
      ) : null}
      {next ? (
        canOfferNext ? (
          <Button
            title={t('chat.reveal.offer', { item: t(next.lower) })}
            loading={step.isPending}
            onPress={() =>
              confirm(
                t('chat.reveal.confirmTitle', { item: t(next.lower) }),
                t('chat.reveal.confirmBody', { alias: otherAlias }),
                t('chat.reveal.offerOk'),
                () => step.mutate(next.level),
              )
            }
          />
        ) : (
          <Button title={t('chat.reveal.addToCard', { item: t(next.lower) })} kind="secondary" onPress={() => router.push('/settings/reveal')} />
        )
      ) : null}
      {r.mine !== 'none' ? (
        <Button
          title={t('chat.reveal.stop')}
          kind="ghost"
          onPress={() => confirm(t('chat.reveal.stopTitle'), t('chat.reveal.stopBody'), t('chat.reveal.stopOk'), () => step.mutate('none'))}
        />
      ) : null}
      <Text variant="caption">{t('chat.reveal.hint')}</Text>
    </View>
  );
}
