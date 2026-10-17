import { FlashList } from '@shopify/flash-list';
import { router, useLocalSearchParams } from 'expo-router';
import { ChevronDown, ChevronUp, PenSquare, Shield } from 'lucide-react-native';
import { useMemo, useState } from 'react';
import { Pressable, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import type { Post } from '@/api/types';
import { BowlIcon } from '@/features/bowls/BowlCard';
import { useBowl, useFeed, useJoin, type Sort } from '@/features/bowls/hooks';
import { ModSheet } from '@/features/bowls/ModSheet';
import { PostCard } from '@/features/bowls/PostCard';
import { SortTabs } from '@/features/bowls/SortTabs';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export default function BowlScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { t, tp } = useT();
  const bowl = useBowl(id);
  const join = useJoin();
  const [sort, setSort] = useState<Sort>('hot');
  const [rules, setRules] = useState(false);
  const feed = useFeed(id, sort);
  const [modding, setModding] = useState<Post | null>(null);
  const pinnedId = feed.data?.pages[0]?.pinned?.id;
  const posts = useMemo(() => {
    const pages = feed.data?.pages ?? [];
    const pinned = pages[0]?.pinned;
    const items = pages.flatMap((p) => p.items);
    return pinned ? [pinned, ...items] : items;
  }, [feed.data]);
  const b = bowl.data;
  const isMod = b?.role === 'owner' || b?.role === 'moderator';

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <ScreenHeader
        title={b?.name}
        right={
          b?.joined ? (
            <Pressable onPress={() => router.push({ pathname: '/compose', params: { bowl: id } })} accessibilityLabel={t('bowls.postInBowl')} hitSlop={10}>
              <PenSquare size={20} color={colors.accent} />
            </Pressable>
          ) : null
        }
      />
      <FlashList
        data={posts}
        keyExtractor={(p) => p.id}
        renderItem={({ item }) => <PostCard p={item} showBowl={false} pinned={item.id === pinnedId && !item.is_prompt} onMod={isMod ? setModding : undefined} />}
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
                    {tp('bowls.members', b.member_count)}
                    {b.line ? ` · ${b.line.name}` : ''}
                  </Text>
                </View>
                <Button title={b.joined ? t('common.joined') : t('common.join')} kind={b.joined ? 'secondary' : 'primary'} className="h-9 px-4" onPress={() => join.mutate({ id: b.id, join: !b.joined })} />
              </View>
              {b.description ? <Text className="text-muted">{b.description}</Text> : null}
              {b.rules_md ? (
                <Pressable onPress={() => setRules(!rules)} className="rounded-xl bg-surface p-3">
                  <View className="flex-row items-center justify-between">
                    <Text variant="label">{t('bowls.rules')}</Text>
                    {rules ? <ChevronUp size={16} color={colors.muted} /> : <ChevronDown size={16} color={colors.muted} />}
                  </View>
                  {rules ? <Text className="mt-2 text-sm leading-6 text-muted">{b.rules_md}</Text> : null}
                </Pressable>
              ) : null}
              <View className="flex-row items-center justify-between">
                {isMod ? (
                  <Pressable onPress={() => router.push({ pathname: '/bowl/mod', params: { id } })} className="flex-row items-center gap-1.5" hitSlop={8}>
                    <Shield size={15} color={colors.accent} />
                    <Text className="text-sm font-medium text-accent">{t('bowls.modTools')}</Text>
                  </Pressable>
                ) : (
                  <View />
                )}
                <SortTabs value={sort} onChange={setSort} />
              </View>
            </View>
          ) : null
        }
        ListEmptyComponent={feed.isPending ? null : <EmptyState title={t('bowls.noPostsTitle')} body={b?.joined ? t('bowls.noPostsJoined') : t('bowls.noPostsJoin')} />}
      />
      <ModSheet bowlId={id} pinnedId={pinnedId} owner={b?.role === 'owner'} post={modding} onClose={() => setModding(null)} />
    </SafeAreaView>
  );
}
