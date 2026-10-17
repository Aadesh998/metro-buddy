import * as Haptics from 'expo-haptics';
import { ArrowBigDown, ArrowBigUp } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export function VoteBar({ score, mine, onVote }: { score: number; mine: -1 | 0 | 1; onVote: (v: -1 | 0 | 1) => void }) {
  const { t } = useT();
  const tap = (v: -1 | 1) => {
    Haptics.selectionAsync().catch(() => {});
    onVote(mine === v ? 0 : v);
  };
  return (
    <View className="flex-row items-center rounded-full bg-raised">
      <Pressable onPress={() => tap(1)} hitSlop={6} className="px-2 py-1" accessibilityLabel={t('bowls.card.upvote')} accessibilityState={{ selected: mine === 1 }}>
        <ArrowBigUp size={20} color={mine === 1 ? colors.brand : colors.muted} fill={mine === 1 ? colors.brand : 'transparent'} />
      </Pressable>
      <Text className={`min-w-6 text-center text-sm font-semibold ${mine === 1 ? 'text-brand' : mine === -1 ? 'text-sky' : 'text-ink'}`}>{score}</Text>
      <Pressable onPress={() => tap(-1)} hitSlop={6} className="px-2 py-1" accessibilityLabel={t('bowls.card.downvote')} accessibilityState={{ selected: mine === -1 }}>
        <ArrowBigDown size={20} color={mine === -1 ? colors.sky : colors.muted} fill={mine === -1 ? colors.sky : 'transparent'} />
      </Pressable>
    </View>
  );
}
