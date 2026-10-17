import { router } from 'expo-router';
import { ArrowRight, Check, Clock } from 'lucide-react-native';
import { View } from 'react-native';

import type { Match } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';
import { useRespondMatch } from './hooks';

// Only the shared stretch is ever shown: never where the other person gets on or off.
export function MatchCard({ m }: { m: Match }) {
  const respond = useRespondMatch();
  const waiting = m.my_status === 'accepted' && m.status === 'proposed';
  return (
    <View className="gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center gap-3">
        <Avatar seed={m.other.avatar_seed} size={44} />
        <View className="flex-1">
          <PersonaName p={m.other} />
          <Text variant="caption">{m.other_mood === 'either' ? 'Up for a chat or quiet' : m.other_mood === 'chatty' ? 'Feeling chatty' : 'Prefers quiet'}</Text>
        </View>
        <View className="rounded-full bg-mint/15 px-2 py-1">
          <Text className="text-xs font-bold text-mint">{Math.round(m.score * 100)}%</Text>
        </View>
      </View>
      <View className="flex-row items-center gap-2 rounded-xl bg-raised px-3 py-2.5">
        <Text className="flex-1 text-sm" numberOfLines={1}>
          {m.segment_from.name}
        </Text>
        <ArrowRight size={14} color={colors.muted} />
        <Text className="flex-1 text-right text-sm" numberOfLines={1}>
          {m.segment_to.name}
        </Text>
      </View>
      <View className="flex-row flex-wrap gap-1.5">
        {m.why.chips.map((c) => (
          <View key={c} className="rounded-full bg-raised px-2.5 py-1">
            <Text className="text-xs text-ink">{c}</Text>
          </View>
        ))}
      </View>
      {m.status === 'accepted' ? (
        <Button title="It's a match. Open chat" onPress={() => m.conversation_id && router.push(`/chat/${m.conversation_id}`)} />
      ) : waiting ? (
        <View className="flex-row items-center justify-center gap-2 py-2">
          <Clock size={14} color={colors.muted} />
          <Text className="text-muted">Waiting for them to say yes</Text>
        </View>
      ) : (
        <View className="flex-row gap-2">
          <Button
            title="Travel together"
            className="flex-1"
            loading={respond.isPending && respond.variables?.accept}
            onPress={() => respond.mutate({ id: m.id, accept: true })}
          />
          <Button title="Not today" kind="secondary" onPress={() => respond.mutate({ id: m.id, accept: false })} />
        </View>
      )}
      {m.status === 'accepted' ? (
        <View className="flex-row items-center gap-1.5">
          <Check size={13} color={colors.mint} />
          <Text variant="caption">Chat only opens when you both agree. Meet inside the station.</Text>
        </View>
      ) : null}
    </View>
  );
}
