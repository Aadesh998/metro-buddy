import { FlashList } from '@shopify/flash-list';
import { router } from 'expo-router';
import { Inbox, KeyRound, MessageCircle, Plus, Radio } from 'lucide-react-native';
import { useState } from 'react';
import { Pressable, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { DMRequest } from '@/api/types';
import { Avatar } from '@/components/Avatar';
import { EmptyState } from '@/components/EmptyState';
import { PushPrompt } from '@/components/PushPrompt';
import { PersonaName } from '@/components/PersonaBadge';
import { TabHeader } from '@/components/TabHeader';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { ConversationRow } from '@/features/chat/ConversationRow';
import { useConversations, useRequests, useRespond } from '@/features/chat/hooks';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { colors } from '@/lib/theme';
import { ago } from '@/lib/time';

function RequestCard({ r }: { r: DMRequest }) {
  const respond = useRespond();
  const safety = useSafetySheet();
  return (
    <View className="mx-4 mb-2 gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="flex-row items-center gap-3">
        <Avatar seed={r.from.avatar_seed} size={40} />
        <View className="flex-1">
          <PersonaName p={r.from} />
          <Text variant="caption">
            via {r.context_type} · {ago(r.created_at)}
          </Text>
        </View>
      </View>
      <Text className="leading-6">“{r.intro}”</Text>
      <View className="flex-row gap-2">
        <Button
          title="Accept"
          className="h-10 flex-1"
          loading={respond.isPending && respond.variables?.id === r.id && respond.variables.action === 'accept'}
          onPress={() => respond.mutate({ id: r.id, action: 'accept' }, { onSuccess: (c) => c && router.push(`/chat/${c.id}`) })}
        />
        <Button title="Ignore" kind="secondary" className="h-10 flex-1" onPress={() => respond.mutate({ id: r.id, action: 'ignore' })} />
        <Button title="Report" kind="ghost" className="h-10" onPress={() => safety.open({ type: 'persona', id: r.from.id, personaId: r.from.id, alias: r.from.alias })} />
      </View>
    </View>
  );
}

export default function ChatsTab() {
  const [tab, setTab] = useState<'chats' | 'requests'>('chats');
  const [menu, setMenu] = useState(false);
  const convs = useConversations();
  const incoming = useRequests('incoming');
  const count = incoming.data?.length ?? 0;

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader
        title="Chats"
        right={
          <View className="flex-row gap-2">
            <Pressable onPress={() => router.push('/inbox')} className="h-10 w-10 items-center justify-center rounded-full bg-raised" accessibilityLabel="Notifications">
              <Inbox size={18} color={colors.ink} />
            </Pressable>
            <Pressable onPress={() => setMenu(!menu)} className="h-10 w-10 items-center justify-center rounded-full bg-accent" accessibilityLabel="New chat">
              <Plus size={20} color={colors.accentInk} />
            </Pressable>
          </View>
        }
      />
      <PushPrompt />
      {menu ? (
        <View className="mx-4 mb-3 overflow-hidden rounded-2xl border border-line bg-surface">
          {[
            { label: 'New group', icon: <MessageCircle size={18} color={colors.ink} />, to: '/groups/new' },
            { label: 'Join with invite code', icon: <KeyRound size={18} color={colors.ink} />, to: '/groups/join' },
            { label: 'Live discussion rooms', icon: <Radio size={18} color={colors.accent} />, to: '/rooms' },
          ].map((m) => (
            <Pressable
              key={m.label}
              onPress={() => {
                setMenu(false);
                router.push(m.to as never);
              }}
              className="flex-row items-center gap-3 px-4 py-3.5 active:bg-raised"
            >
              {m.icon}
              <Text>{m.label}</Text>
            </Pressable>
          ))}
        </View>
      ) : null}
      <View className="mx-4 mb-3 flex-row rounded-xl bg-raised p-1">
        {(['chats', 'requests'] as const).map((t) => (
          <Pressable key={t} onPress={() => setTab(t)} className={`flex-1 flex-row items-center justify-center gap-1.5 rounded-lg py-2 ${tab === t ? 'bg-bg' : ''}`}>
            <Text className={`text-sm font-semibold capitalize ${tab === t ? 'text-ink' : 'text-muted'}`}>{t}</Text>
            {t === 'requests' && count > 0 ? (
              <View className="rounded-full bg-accent px-1.5">
                <Text className="text-[11px] font-bold text-accent-ink">{count}</Text>
              </View>
            ) : null}
          </Pressable>
        ))}
      </View>
      {tab === 'chats' ? (
        <FlashList
          data={convs.data ?? []}
          keyExtractor={(c) => c.id}
          renderItem={({ item }) => <ConversationRow c={item} />}
          refreshControl={<RefreshControl refreshing={convs.isRefetching} onRefresh={() => void convs.refetch()} tintColor={colors.accent} />}
          ListEmptyComponent={
            convs.isPending ? null : (
              <EmptyState icon={<MessageCircle color={colors.accent} size={36} />} title="No chats yet" body="Chats start when someone accepts your request, or you join a group or room." />
            )
          }
        />
      ) : (
        <FlashList
          data={incoming.data ?? []}
          keyExtractor={(r) => r.id}
          renderItem={({ item }) => <RequestCard r={item} />}
          ListEmptyComponent={incoming.isPending ? null : <EmptyState title="No requests" body="Nobody can message you without your yes." />}
        />
      )}
    </SafeAreaView>
  );
}
