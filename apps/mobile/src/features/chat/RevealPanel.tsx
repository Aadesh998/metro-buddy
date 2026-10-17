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
import { confirm } from '@/lib/confirm';
import { colors } from '@/lib/theme';

const STEPS: { level: RevealLevel; label: string }[] = [
  { level: 'first_name', label: 'First name' },
  { level: 'photo', label: 'Photo' },
  { level: 'contact', label: 'Contact' },
];
const order: Record<RevealLevel, number> = { none: 0, first_name: 1, photo: 2, contact: 3 };

export function RevealPanel({ conversationId, otherAlias }: { conversationId: string; otherAlias: string }) {
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
        <Eye size={18} color={colors.accent} />
        <Text variant="label">Reveal, step by step</Text>
      </View>
      <View className="flex-row gap-2">
        {STEPS.map((s) => {
          const mutual = order[r.mutual] >= order[s.level];
          const mine = order[r.mine] >= order[s.level];
          const theirs = order[r.theirs] >= order[s.level];
          return (
            <View key={s.level} className={`flex-1 items-center gap-1 rounded-xl p-2 ${mutual ? 'bg-mint/15' : 'bg-raised'}`}>
              <Text className="text-xs font-semibold">{s.label}</Text>
              <Text className="text-[10px] text-muted">{mutual ? 'shared' : mine ? 'waiting for them' : theirs ? 'they’re ready' : '—'}</Text>
            </View>
          );
        })}
      </View>
      {r.mutual !== 'none' ? (
        <View className="flex-row items-center gap-3 rounded-xl bg-raised p-3">
          {r.revealed.photo_url ? <Image source={{ uri: r.revealed.photo_url }} style={{ width: 48, height: 48, borderRadius: 24 }} /> : null}
          <View className="flex-1">
            <Text variant="label">{r.revealed.first_name}</Text>
            {r.revealed.contact ? <Text selectable className="text-sm text-accent">{r.revealed.contact}</Text> : null}
          </View>
          <Check size={16} color={colors.mint} />
        </View>
      ) : null}
      {next ? (
        canOfferNext ? (
          <Button
            title={`Offer to share ${next.label.toLowerCase()}`}
            loading={step.isPending}
            onPress={() =>
              confirm(
                `Share your ${next.label.toLowerCase()}?`,
                `${otherAlias} will only see it if they share theirs too. You can stop sharing any time.`,
                'Offer',
                () => step.mutate(next.level),
              )
            }
          />
        ) : (
          <Button title={`Add your ${next.label.toLowerCase()} to your reveal card`} kind="secondary" onPress={() => router.push('/settings/reveal')} />
        )
      ) : null}
      {r.mine !== 'none' ? (
        <Button
          title="Stop sharing"
          kind="ghost"
          onPress={() => confirm('Stop sharing?', 'They won’t see your details anymore, and contact details in new messages are hidden again.', 'Stop', () => step.mutate('none'))}
        />
      ) : null}
      <Text variant="caption">Only reveal to people you trust. If anything feels off, report or block from the menu.</Text>
    </View>
  );
}
