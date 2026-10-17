import { FlashList } from '@shopify/flash-list';
import { router, useLocalSearchParams } from 'expo-router';
import { ChevronDown, ChevronUp, PenSquare } from 'lucide-react-native';
import { useMemo, useState } from 'react';
import { Pressable, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { BowlIcon } from '@/features/bowls/BowlCard';
import { useBowl, useFeed, useJoin, type Sort } from '@/features/bowls/hooks';
import { PostCard } from '@/features/bowls/PostCard';
import { SortTabs } from '@/features/bowls/SortTabs';
import { colors } from '@/lib/theme';

export default function BowlScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const bowl = useBowl(id);
  const join = useJoin();
  const [sort, setSort] = useState<Sort>('hot');
  const [rules, setRules] = useState(false);
  const feed = useFeed(id, sort);
  const posts = useMemo(() => {
    const pages = feed.data?.pages ?? [];
    const pinned = pages[0]?.pinned;
    const items = pages.flatMap((p) => p.items);
    return pinned ? [pinned, ...items] : items;
  }, [feed.data]);
  const b = bowl.data;

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <ScreenHeader
        title={b?.name}
        right={
          b?.joined ? (
            <Pressable onPress={() => router.push({ pathname: '/compose', params: { bowl: id } })} accessibilityLabel="Post in this bowl" hitSlop={10}>
              <PenSquare size={20} color={colors.accent} />
            </Pressable>
          ) : null
        }
      />
      <FlashList
        data={posts}
        keyExtractor={(p) => p.id}
        renderItem={({ item }) => <PostCard p={item} showBowl={false} />}
        onEndReached={() => feed.hasNextPage && !feed.isFetchingNextPage && feed.fetchNextPage()}
        refreshControl={<RefreshControl refreshing={feed.isRefetching} onRefresh={() => void feed.refetch()} tintColor={colors.accent} />}
        ListHeaderComponent={
          b ? (
            <View className="mb-2 gap-3 border-b border-line px-4 pb-4">
              <View className="flex-row items-center gap-3">
                <BowlIcon b={b} />
                <View className="flex-1">
                  <Text variant="heading">{b.name}</Text>
                  <Text variant="caption">
                    {b.member_count} member{b.member_count === 1 ? '' : 's'}
                    {b.line ? ` · ${b.line.name}` : ''}
                  </Text>
                </View>
                <Button title={b.joined ? 'Joined' : 'Join'} kind={b.joined ? 'secondary' : 'primary'} className="h-9 px-4" onPress={() => join.mutate({ id: b.id, join: !b.joined })} />
              </View>
              {b.description ? <Text className="text-muted">{b.description}</Text> : null}
              {b.rules_md ? (
                <Pressable onPress={() => setRules(!rules)} className="rounded-xl bg-surface p-3">
                  <View className="flex-row items-center justify-between">
                    <Text variant="label">Bowl rules</Text>
                    {rules ? <ChevronUp size={16} color={colors.muted} /> : <ChevronDown size={16} color={colors.muted} />}
                  </View>
                  {rules ? <Text className="mt-2 text-sm leading-6 text-muted">{b.rules_md}</Text> : null}
                </Pressable>
              ) : null}
              <View className="flex-row justify-end">
                <SortTabs value={sort} onChange={setSort} />
              </View>
            </View>
          ) : null
        }
        ListEmptyComponent={feed.isPending ? null : <EmptyState title="No posts yet" body={b?.joined ? 'Start the conversation.' : 'Join to post here.'} />}
      />
    </SafeAreaView>
  );
}
