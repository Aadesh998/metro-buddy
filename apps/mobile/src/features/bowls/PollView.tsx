import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, View } from 'react-native';

import { post } from '@/api/client';
import type { Post } from '@/api/types';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';
import { until } from '@/lib/time';
import { patchPost } from './hooks';

export function PollView({ p }: { p: Post }) {
  const qc = useQueryClient();
  const poll = p.poll!;
  const voted = poll.options.some((o) => o.mine);
  const showResults = voted || poll.closed;
  const [picked, setPicked] = useState<string[]>([]);
  const vote = useMutation({
    mutationFn: () => post<Post>(`/v1/posts/${p.id}/poll/vote`, { option_ids: picked }),
    onSuccess: (np) => patchPost(qc, p.id, () => np),
  });

  return (
    <View className="mt-3 gap-2">
      {poll.options.map((o) => {
        const pct = poll.total ? Math.round((o.votes / poll.total) * 100) : 0;
        const sel = picked.includes(o.id);
        return (
          <Pressable
            key={o.id}
            disabled={showResults}
            onPress={() => setPicked((xs) => (poll.multi ? (sel ? xs.filter((x) => x !== o.id) : [...xs, o.id]) : [o.id]))}
            className={`overflow-hidden rounded-xl border ${sel || o.mine ? 'border-accent' : 'border-line'} bg-surface`}
            accessibilityState={{ selected: sel || o.mine }}
          >
            {showResults ? <View style={{ width: `${pct}%` }} className="absolute bottom-0 left-0 top-0 bg-accent/15" /> : null}
            <View className="flex-row items-center justify-between px-3 py-2.5">
              <Text className="flex-1 text-sm">{o.label}</Text>
              {o.mine ? <Check size={14} color={colors.accent} /> : null}
              {showResults ? <Text className="ml-2 text-sm font-semibold">{pct}%</Text> : null}
            </View>
          </Pressable>
        );
      })}
      <View className="flex-row items-center justify-between">
        <Text variant="caption">
          {poll.total} vote{poll.total === 1 ? '' : 's'} · {until(poll.closes_at)}
        </Text>
        {!showResults ? <Button title="Vote" className="h-9" disabled={picked.length === 0} loading={vote.isPending} onPress={() => vote.mutate()} /> : null}
      </View>
    </View>
  );
}
