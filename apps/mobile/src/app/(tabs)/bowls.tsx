import { FlashList } from '@shopify/flash-list';
import { router } from 'expo-router';
import { Coffee, PenSquare, Search } from 'lucide-react-native';
import { useMemo, useState } from 'react';
import { Pressable, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { FontTextInput } from '@/components/ui/FontTextInput';
import type { Post } from '@/api/types';
import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { BowlCard } from '@/features/bowls/BowlCard';
import { useBowlSearch, useFeed, useJoinedBowls, useSuggestedBowls, type Sort } from '@/features/bowls/hooks';
import { PostCard } from '@/features/bowls/PostCard';
import { SortTabs } from '@/features/bowls/SortTabs';
import { useMe } from '@/features/me/hooks';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

function ForYou({ onDiscover }: { onDiscover: () => void }) {
  const { t } = useT();
  const [sort, setSort] = useState<Sort>('hot');
  const feed = useFeed('home', sort);
  const joined = useJoinedBowls();
  const posts = useMemo(() => {
    const pages = feed.data?.pages ?? [];
    const pinned = pages[0]?.pinned;
    const items = pages.flatMap((p) => p.items);
    return pinned ? [pinned, ...items] : items;
  }, [feed.data]);

  if (joined.data && joined.data.length === 0) {
    return (
      <EmptyState
        icon={<Coffee color={colors.accent} size={36} />}
        title={t('bowls.emptyFeedTitle')}
        body={t('bowls.emptyFeedBody')}
        action={<Button title={t('bowls.findBowls')} onPress={onDiscover} />}
      />
    );
  }
  return (
    <FlashList
      data={posts}
      keyExtractor={(p: Post) => p.id}
      renderItem={({ item }) => <PostCard p={item} />}
      ListHeaderComponent={
        <View className="flex-row justify-end px-4 pb-2">
          <SortTabs value={sort} onChange={setSort} />
        </View>
      }
      onEndReached={() => feed.hasNextPage && !feed.isFetchingNextPage && feed.fetchNextPage()}
      onEndReachedThreshold={0.5}
      refreshControl={<RefreshControl refreshing={feed.isRefetching} onRefresh={() => void feed.refetch()} tintColor={colors.accent} />}
      ListEmptyComponent={feed.isPending ? null : <EmptyState title={t('bowls.quietTitle')} body={t('bowls.quietBody')} />}
    />
  );
}

function Discover() {
  const { t } = useT();
  const me = useMe();
  const [q, setQ] = useState('');
  const suggested = useSuggestedBowls();
  const search = useBowlSearch(q);
  const list = q.trim().length >= 2 ? (search.data ?? []) : (suggested.data ?? []);
  return (
    <View className="flex-1">
      <View className="mx-4 mb-3 h-11 flex-row items-center gap-2 rounded-xl border border-line bg-surface px-3">
        <Search color={colors.faint} size={18} />
        <FontTextInput value={q} onChangeText={setQ} placeholder={t('bowls.searchPlaceholder')} placeholderTextColor={colors.faint} className="flex-1 text-base text-ink" />
      </View>
      <FlashList
        data={list}
        keyExtractor={(b) => b.id}
        contentContainerStyle={{ paddingHorizontal: 16, paddingBottom: 24 }}
        ItemSeparatorComponent={() => <View className="h-2" />}
        ListHeaderComponent={
          !q ? (
            <View className="gap-3">
              {(me.data?.trust_tier ?? 0) >= 2 ? <Button title={t('bowls.startBowl')} kind="secondary" onPress={() => router.push('/bowl/new')} /> : null}
              <Text variant="caption" className="mb-2 uppercase tracking-wider">
                {t('bowls.suggested')}
              </Text>
            </View>
          ) : null
        }
        renderItem={({ item }) => <BowlCard b={item} />}
        ListEmptyComponent={q.length >= 2 && !search.isPending ? <Text className="py-8 text-center text-muted">{t('bowls.noMatch', { q })}</Text> : null}
      />
    </View>
  );
}

export default function BowlsTab() {
  const { t } = useT();
  const [tab, setTab] = useState<'feed' | 'discover'>('feed');
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader
        title={t('common.tabs.bowls')}
        right={
          <Pressable onPress={() => router.push('/compose')} className="h-10 w-10 items-center justify-center rounded-full bg-accent" accessibilityLabel={t('bowls.newPost')}>
            <PenSquare size={18} color={colors.accentInk} />
          </Pressable>
        }
      />
      <View className="mx-4 mb-3 flex-row rounded-xl bg-raised p-1">
        {(['feed', 'discover'] as const).map((k) => (
          <Pressable key={k} onPress={() => setTab(k)} className={`flex-1 items-center rounded-lg py-2 ${tab === k ? 'bg-bg' : ''}`} accessibilityState={{ selected: tab === k }}>
            <Text className={`text-sm font-semibold ${tab === k ? 'text-ink' : 'text-muted'}`}>{k === 'feed' ? t('bowls.forYou') : t('bowls.discover')}</Text>
          </Pressable>
        ))}
      </View>
      {tab === 'feed' ? <ForYou onDiscover={() => setTab('discover')} /> : <Discover />}
    </SafeAreaView>
  );
}
