import { FlashList } from '@shopify/flash-list';
import { router } from 'expo-router';
import { Plus } from 'lucide-react-native';
import { useState } from 'react';
import { Modal, Pressable, RefreshControl, ScrollView, View } from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';

import { ApiError, post } from '@/api/client';
import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useCatalog, useRooms, type GameName, type Room, type SizeFilter } from '@/features/games/api';
import { GAMES, GameIcon } from '@/features/games/catalog';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

function GameCard({ name, onPress }: { name: GameName; onPress: () => void }) {
  const { t } = useT();
  return (
    <Pressable onPress={onPress} className="w-[48%] gap-2 rounded-2xl border border-line bg-surface p-4 active:bg-raised" accessibilityRole="button">
      <View className="h-10 w-10 items-center justify-center rounded-full bg-brand-soft">
        <GameIcon name={name} size={20} color={colors.brand} />
      </View>
      <Text variant="label" numberOfLines={1}>
        {t(`games.names.${name}`)}
      </Text>
      <Text variant="caption" numberOfLines={3}>
        {t(`games.blurbs.${name}`)}
      </Text>
    </Pressable>
  );
}

function RoomRow({ r }: { r: Room }) {
  const { t } = useT();
  const full = r.player_count >= r.max_players || r.status !== 'lobby';
  const host = r.players.find((p) => p.persona.id === r.host_id)?.persona.alias;
  const open = (spectator: boolean) =>
    post<Room>(`/v1/games/rooms/${r.id}/join`, { spectator })
      .then(() => router.push(`/game/${r.id}`))
      .catch((e) => toast(e instanceof ApiError ? e.message : String(e), 'error'));
  return (
    <View className="mx-4 mb-2 flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-3">
      <View className="h-10 w-10 items-center justify-center rounded-full bg-raised">
        <GameIcon name={r.game} size={18} color={colors.ink} />
      </View>
      <View className="min-w-0 flex-1">
        <Text variant="label" numberOfLines={1}>
          {t(`games.names.${r.game}`)}
        </Text>
        <Text variant="caption" numberOfLines={1}>
          {[host, t('games.playersOf', { n: r.player_count, max: r.max_players }), t(`games.status.${r.status}`)].filter(Boolean).join(' · ')}
        </Text>
      </View>
      <Pressable
        onPress={() => open(full)}
        className={`h-9 justify-center rounded-full px-4 ${full ? 'border border-line' : 'bg-accent'}`}
        accessibilityRole="button"
      >
        <Text className={`text-sm font-semibold ${full ? 'text-ink' : 'text-accent-ink'}`}>{full ? t('games.watch') : t('games.join')}</Text>
      </Pressable>
    </View>
  );
}

function NewRoomSheet({ game, onClose }: { game: GameName | null; onClose: () => void }) {
  const { t, tp } = useT();
  const insets = useSafeAreaInsets();
  const catalog = useCatalog();
  const info = catalog.data?.find((g) => g.name === game);
  const [vis, setVis] = useState<'public' | 'private'>('public');
  const [max, setMax] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const sizes = info ? Array.from(new Set([info.min_players, 2, 4, 6, 8, info.max_players])).filter((n) => n >= Math.max(2, info.min_players) && n <= info.max_players) : [];
  const chosen = max ?? info?.max_players ?? 2;
  const create = () => {
    setBusy(true);
    post<Room>('/v1/games/rooms', { game, visibility: vis, max_players: chosen })
      .then((r) => {
        onClose();
        router.push(`/game/${r.id}`);
      })
      .catch((e) => toast(e instanceof ApiError ? e.message : String(e), 'error'))
      .finally(() => setBusy(false));
  };
  return (
    <Modal visible={!!game} transparent animationType="slide" onRequestClose={onClose}>
      <Pressable className="flex-1 bg-black/60" onPress={onClose} accessibilityLabel={t('common.close')} />
      <View className="gap-4 rounded-t-2xl border-t border-line bg-surface px-5 pt-3" style={{ paddingBottom: insets.bottom + 16 }}>
        <View className="h-1 w-10 self-center rounded-full bg-line" />
        <Text variant="heading">{game ? t(`games.names.${game}`) : ''}</Text>
        <View className="gap-2">
          <Text variant="label">{t('games.newRoom.who')}</Text>
          <View className="flex-row gap-2">
            <Chip label={t('games.newRoom.public')} selected={vis === 'public'} onPress={() => setVis('public')} />
            <Chip label={t('games.newRoom.private')} selected={vis === 'private'} onPress={() => setVis('private')} />
          </View>
        </View>
        {sizes.length > 1 ? (
          <View className="gap-2">
            <Text variant="label">{t('games.newRoom.maxPlayers')}</Text>
            <View className="flex-row flex-wrap gap-2">
              {sizes.map((n) => (
                <Chip key={n} label={tp('games.players', n)} selected={chosen === n} onPress={() => setMax(n)} />
              ))}
            </View>
          </View>
        ) : null}
        <Button title={t('games.create')} loading={busy} onPress={create} />
      </View>
    </Modal>
  );
}

export default function GamesTab() {
  const { t } = useT();
  const catalog = useCatalog();
  const [size, setSize] = useState<SizeFilter | undefined>();
  const rooms = useRooms(undefined, size);
  const [code, setCode] = useState('');
  const [creating, setCreating] = useState<GameName | null>(null);
  const playable = new Set(catalog.data?.map((g) => g.name) ?? []);
  const joinByCode = () =>
    post<Room>('/v1/games/rooms/join', { code })
      .then((r) => {
        setCode('');
        router.push(`/game/${r.id}`);
      })
      .catch((e) => toast(e instanceof ApiError ? e.message : String(e), 'error'));

  const header = (
    <View className="gap-5 pb-3">
      <Text className="px-5 text-muted">{t('games.subtitle')}</Text>
      <View className="flex-row flex-wrap justify-between gap-y-3 px-4">
        {GAMES.filter((g) => playable.has(g.name)).map((g) => (
          <GameCard key={g.name} name={g.name} onPress={() => setCreating(g.name)} />
        ))}
      </View>
      <View className="flex-row items-end gap-2 px-4">
        <View className="flex-1">
          <Input
            label={t('games.joinCode')}
            value={code}
            onChangeText={(v) => setCode(v.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 6))}
            placeholder={t('games.joinCodePlaceholder')}
            autoCapitalize="characters"
            autoCorrect={false}
          />
        </View>
        <Button title={t('games.join')} disabled={code.length < 6} onPress={joinByCode} />
      </View>
      <View className="gap-2">
        <Text variant="heading" className="px-5">
          {t('games.publicRooms')}
        </Text>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2 px-4">
          <Chip label={t('games.size.all')} selected={!size} onPress={() => setSize(undefined)} />
          {(['small', 'group', 'party'] as const).map((s) => (
            <Chip key={s} label={t(`games.size.${s}`)} selected={size === s} onPress={() => setSize(s)} />
          ))}
        </ScrollView>
      </View>
    </View>
  );

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader
        title={t('games.title')}
        right={
          <Pressable
            onPress={() => setCreating(GAMES.find((g) => playable.has(g.name))?.name ?? null)}
            className="h-10 w-10 items-center justify-center rounded-full bg-accent"
            accessibilityLabel={t('games.create')}
          >
            <Plus size={20} color={colors.accentInk} />
          </Pressable>
        }
      />
      <FlashList
        data={rooms.data ?? []}
        keyExtractor={(r) => r.id}
        renderItem={({ item }) => <RoomRow r={item} />}
        ListHeaderComponent={header}
        keyboardShouldPersistTaps="handled"
        refreshControl={<RefreshControl refreshing={rooms.isRefetching} onRefresh={() => void rooms.refetch()} tintColor={colors.brand} />}
        ListEmptyComponent={rooms.isPending ? null : <EmptyState title={t('games.noRooms')} />}
      />
      <NewRoomSheet game={creating} onClose={() => setCreating(null)} />
    </SafeAreaView>
  );
}
