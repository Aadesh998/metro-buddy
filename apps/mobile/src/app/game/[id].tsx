import { router, useLocalSearchParams } from 'expo-router';
import { Crown, Eye, LogOut, Share2 } from 'lucide-react-native';
import { Alert, Pressable, ScrollView, Share, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Avatar } from '@/components/Avatar';
import { EmptyState } from '@/components/EmptyState';
import { PersonaName } from '@/components/PersonaBadge';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { useMove, useRoom, useRoomAction, type Room, type RoomPlayer } from '@/features/games/api';
import { BOARDS } from '@/features/games/boards';
import { useNow } from '@/features/games/useNow';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

function PlayerRow({ p, host, showReady, showScore }: { p: RoomPlayer; host: boolean; showReady?: boolean; showScore?: boolean }) {
  const { t, tp } = useT();
  return (
    <View className="flex-row items-center gap-3 py-2">
      <Avatar seed={p.persona.avatar_seed} size={36} />
      <View className="min-w-0 flex-1 flex-row items-center gap-1.5">
        <PersonaName p={p.persona} />
        {host ? <Crown size={14} color={colors.brand} accessibilityLabel={t('games.lobby.host')} /> : null}
      </View>
      {showReady ? (
        <Text className={`text-xs font-semibold ${p.ready || host ? 'text-mint' : 'text-faint'}`}>
          {p.ready || host ? t('games.lobby.ready') : t('games.lobby.notReady')}
        </Text>
      ) : null}
      {showScore ? <Text className="text-sm font-semibold">{tp('games.results.points', p.score)}</Text> : null}
    </View>
  );
}

function Lobby({ room, myId }: { room: Room; myId?: string }) {
  const { t } = useT();
  const act = useRoomAction(room.id);
  const players = room.players.filter((p) => !p.spectator);
  const watchers = room.players.filter((p) => p.spectator);
  const isHost = room.host_id === myId;
  const me = room.me;
  const allReady = players.every((p) => p.ready || p.persona.id === room.host_id);
  const enough = players.length >= room.min_players;
  const name = t(`games.names.${room.game}`);
  return (
    <ScrollView contentContainerClassName="gap-5 px-5 pb-10">
      {room.code ? (
        <View className="items-center gap-2 rounded-2xl border border-line bg-surface p-5">
          <Text variant="caption">{t('games.lobby.code')}</Text>
          <Text selectable className="font-mono text-3xl font-bold tracking-[6px]">
            {room.code}
          </Text>
          <Pressable
            onPress={() => Share.share({ message: t('games.lobby.shareMessage', { game: name, code: room.code ?? '' }) })}
            className="flex-row items-center gap-2 rounded-full border border-line px-4 py-2 active:bg-raised"
            accessibilityRole="button"
          >
            <Share2 size={16} color={colors.ink} />
            <Text className="text-sm font-semibold">{t('games.lobby.share')}</Text>
          </Pressable>
        </View>
      ) : null}

      <View>
        <Text variant="label" className="mb-1">
          {t('games.lobby.players')} · {t('games.playersOf', { n: players.length, max: room.max_players })}
        </Text>
        {players.map((p) => (
          <PlayerRow key={p.persona.id} p={p} host={p.persona.id === room.host_id} showReady />
        ))}
      </View>
      {watchers.length ? (
        <View>
          <Text variant="label" className="mb-1">
            {t('games.lobby.spectators')}
          </Text>
          {watchers.map((p) => (
            <PlayerRow key={p.persona.id} p={p} host={false} />
          ))}
        </View>
      ) : null}

      {me?.spectator ? (
        <Text className="text-center text-muted">{t('games.lobby.spectating')}</Text>
      ) : isHost ? (
        <View className="gap-2">
          <Button title={t('games.lobby.start')} disabled={!enough || !allReady} loading={act.isPending} onPress={() => act.mutate({ action: 'start' })} />
          <Text variant="caption" className="text-center">
            {!enough ? t('games.lobby.needPlayers', { n: room.min_players }) : !allReady ? t('games.lobby.waitingReady') : ''}
          </Text>
        </View>
      ) : me ? (
        <View className="gap-2">
          <Button
            title={me.ready ? t('games.lobby.unready') : t('games.lobby.imReady')}
            kind={me.ready ? 'secondary' : 'primary'}
            loading={act.isPending}
            onPress={() => act.mutate({ action: 'ready', body: { ready: !me.ready } })}
          />
          <Text variant="caption" className="text-center">
            {t('games.lobby.waitingHost')}
          </Text>
        </View>
      ) : null}
    </ScrollView>
  );
}

function Results({ room, myId }: { room: Room; myId?: string }) {
  const { t } = useT();
  const act = useRoomAction(room.id);
  const ranked = room.players.filter((p) => !p.spectator).sort((a, b) => b.score - a.score);
  return (
    <View className="gap-3 px-5 pb-6 pt-2">
      <Text variant="heading">{t('games.results.title')}</Text>
      {ranked.map((p) => (
        <PlayerRow key={p.persona.id} p={p} host={false} showScore />
      ))}
      {room.host_id === myId ? <Button title={t('games.results.rematch')} loading={act.isPending} onPress={() => act.mutate({ action: 'rematch' })} /> : null}
      <Button title={t('games.results.back')} kind="secondary" onPress={() => router.back()} />
    </View>
  );
}

export default function GameRoomScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { t } = useT();
  const myId = useMyPersona().data?.id;
  const q = useRoom(id);
  const act = useRoomAction(id);
  const mv = useMove(id);
  const now = useNow(500);
  const room = q.data;
  const Board = room ? BOARDS[room.game] : undefined;

  const leave = () =>
    Alert.alert(t('games.lobby.leave'), undefined, [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('games.lobby.leave'), style: 'destructive', onPress: () => act.mutate({ action: 'leave' }, { onSuccess: () => router.back() }) },
    ]);

  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <ScreenHeader
        title={room ? t(`games.names.${room.game}`) : ''}
        right={
          room?.me ? (
            <Pressable onPress={leave} hitSlop={12} accessibilityLabel={t('games.lobby.leave')}>
              <LogOut size={20} color={colors.ink} />
            </Pressable>
          ) : null
        }
      />
      {q.isError ? (
        <EmptyState title={t('bowls.profile.unavailableTitle')} />
      ) : !room ? null : room.status === 'lobby' ? (
        <Lobby room={room} myId={myId} />
      ) : room.status === 'closed' ? (
        <EmptyState title={t('games.status.closed')} />
      ) : (
        <ScrollView contentContainerClassName="gap-4 pb-6" keyboardShouldPersistTaps="handled">
          {room.me?.spectator ? (
            <View className="flex-row items-center justify-center gap-2">
              <Eye size={14} color={colors.muted} />
              <Text variant="caption">{t('games.lobby.spectating')}</Text>
            </View>
          ) : null}
          {Board && room.view ? (
            <Board room={room} view={room.view} me={room.me} myId={myId} spectator={!room.me || room.me.spectator} move={(m) => mv.mutate(m)} moving={mv.isPending} now={now} />
          ) : (
            <EmptyState title={t('games.updateApp')} />
          )}
          {room.status === 'finished' ? <Results room={room} myId={myId} /> : null}
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
