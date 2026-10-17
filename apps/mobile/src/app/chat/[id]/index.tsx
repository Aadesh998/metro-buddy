import { FlashList } from '@shopify/flash-list';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Clipboard from 'expo-clipboard';
import { router, useLocalSearchParams } from 'expo-router';
import { ChevronLeft, Info, MoreHorizontal, Send } from 'lucide-react-native';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Alert, KeyboardAvoidingView, Platform, Pressable, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { del, post } from '@/api/client';
import type { Conversation, Message } from '@/api/types';
import { SupportSheet } from '@/components/SupportSheet';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { useConversationTitle } from '@/features/chat/ConversationRow';
import { chatKeys, useConversation, useMessages, useSend } from '@/features/chat/hooks';
import { MessageBubble } from '@/features/chat/MessageBubble';
import { activeConversation, useOnline, useReceipts, useSupportSignal, useTyping } from '@/features/chat/store';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { send as frame } from '@/lib/realtime';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';
import { until } from '@/lib/time';

function Header({ c, id }: { c?: Conversation; id: string }) {
  const { title, other } = useConversationTitle(c ?? ({ members: [], kind: 'group', title: '' } as unknown as Conversation));
  const online = useOnline(other?.id);
  const typing = useTyping(id);
  const safety = useSafetySheet();
  let sub = '';
  if (typing.length) sub = c?.kind === 'dm' ? 'typing…' : `${typing.length} typing…`;
  else if (c?.kind === 'discussion' && c.ends_at) sub = c.closed ? 'Room closed' : `Live · ${until(c.ends_at)}`;
  else if (c && c.kind !== 'dm' && c.kind !== 'travel') sub = `${c.member_count} members`;
  else if (online) sub = 'online';
  return (
    <View className="h-14 flex-row items-center border-b border-line px-2">
      <Pressable onPress={() => router.back()} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel="Back">
        <ChevronLeft size={26} color={colors.ink} />
      </Pressable>
      <View className="flex-1">
        <Text className="font-semibold" numberOfLines={1}>
          {c ? title : ''}
        </Text>
        {sub ? <Text className={`text-xs ${typing.length ? 'text-accent' : 'text-muted'}`}>{sub}</Text> : null}
      </View>
      {c && (c.kind === 'dm' || c.kind === 'travel') && other ? (
        <Pressable onPress={() => safety.open({ type: 'conversation', id, personaId: other.id, alias: other.alias })} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel="Report, block or mute">
          <MoreHorizontal size={22} color={colors.ink} />
        </Pressable>
      ) : null}
      {c ? (
        <Pressable onPress={() => router.push(`/chat/${id}/info`)} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel="Chat info">
          <Info size={20} color={colors.ink} />
        </Pressable>
      ) : null}
    </View>
  );
}

export default function ChatScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const conv = useConversation(id);
  const msgs = useMessages(id);
  const send = useSend(id);
  const safety = useSafetySheet();
  const receipts = useReceipts(id);
  const support = useSupportSignal();
  const [supportSeen, setSupportSeen] = useState(support);
  const [text, setText] = useState('');
  const lastTyping = useRef(0);
  const now = useNow(15_000);
  const items = useMemo(() => (msgs.data?.pages.flatMap((p) => p.items) ?? []).filter((m) => !m.expires_at || Date.parse(m.expires_at) > now), [msgs.data, now]);
  const c = conv.data;
  const isGroup = c ? c.kind !== 'dm' && c.kind !== 'travel' : false;

  useEffect(() => {
    activeConversation.set(id);
    return () => activeConversation.set(null);
  }, [id]);

  const newestIncoming = items.find((m) => !m.is_mine && !m.local)?.id;
  useEffect(() => {
    if (!newestIncoming) return;
    post(`/v1/conversations/${id}/read`, { message_id: newestIncoming }).catch(() => {});
    qc.setQueryData<Conversation[]>(chatKeys.conversations, (old) => old?.map((x) => (x.id === id ? { ...x, unread: 0 } : x)));
  }, [newestIncoming, id, qc]);

  // "Seen" sits under my newest message once the other person's read pointer reaches it.
  const seenId = useMemo(() => {
    if (!c || isGroup) return undefined;
    const partnerRead = Object.values(receipts)[0]?.read;
    const newestMine = items.find((m) => m.is_mine && !m.local);
    if (!partnerRead || !newestMine) return undefined;
    const readIdx = items.findIndex((m) => m.id === partnerRead);
    const mineIdx = items.indexOf(newestMine);
    return readIdx !== -1 && readIdx <= mineIdx ? newestMine.id : undefined;
  }, [receipts, items, c, isGroup]);

  const join = useMutation({
    mutationFn: () => post(`/v1/rooms/${id}/join`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: chatKeys.conversation(id) });
      qc.invalidateQueries({ queryKey: chatKeys.messages(id) });
    },
  });

  function onLongPress(m: Message) {
    const actions: { text: string; style?: 'destructive' | 'cancel'; onPress?: () => void }[] = [];
    if (m.body && !m.deleted) actions.push({ text: 'Copy', onPress: () => Clipboard.setStringAsync(m.body) });
    if (m.is_mine && !m.local) actions.push({ text: 'Delete for everyone', style: 'destructive', onPress: () => del(`/v1/messages/${m.id}`) });
    if (!m.is_mine && m.sender) actions.push({ text: 'Report or block', onPress: () => safety.open({ type: 'message', id: m.id, personaId: m.sender!.id, alias: m.sender!.alias }) });
    actions.push({ text: 'Cancel', style: 'cancel' });
    Alert.alert('Message', undefined, actions);
  }

  function onChange(t: string) {
    setText(t.slice(0, 4000));
    if (Date.now() - lastTyping.current > 2500) {
      lastTyping.current = Date.now();
      frame('typing', { conversation_id: id });
    }
  }

  async function submit() {
    const body = text.trim();
    if (!body) return;
    setText('');
    await send(body);
  }

  const notJoined = conv.isError && !c;
  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <Header c={c} id={id} />
      <KeyboardAvoidingView className="flex-1" behavior={Platform.OS === 'ios' ? 'padding' : undefined} keyboardVerticalOffset={0}>
        {notJoined ? (
          <View className="flex-1 items-center justify-center gap-3 px-8">
            <Text className="text-center text-muted">You’re not in this conversation.</Text>
            <Button title="Join room" onPress={() => join.mutate()} loading={join.isPending} />
          </View>
        ) : (
          <FlashList
            data={items}
            inverted
            keyExtractor={(m) => m.id}
            renderItem={({ item, index }) => (
              <MessageBubble
                m={item}
                showSender={isGroup && items[index + 1]?.sender?.id !== item.sender?.id}
                seen={item.id === seenId}
                onLongPress={onLongPress}
                onRetry={(m) => send(m.body)}
                onPickOpener={setText}
              />
            )}
            onEndReached={() => msgs.hasNextPage && !msgs.isFetchingNextPage && msgs.fetchNextPage()}
            onEndReachedThreshold={0.3}
            contentContainerStyle={{ paddingVertical: 8 }}
          />
        )}
        {c?.closed ? (
          <View className="border-t border-line p-4">
            <Text className="text-center text-muted">This conversation is closed.</Text>
          </View>
        ) : c ? (
          <View className="flex-row items-end gap-2 border-t border-line bg-surface px-3 py-2">
            <TextInput
              value={text}
              onChangeText={onChange}
              placeholder={c.slow_mode_sec ? `Slow mode: one message every ${c.slow_mode_sec}s` : 'Message'}
              placeholderTextColor={colors.faint}
              multiline
              className="max-h-32 flex-1 rounded-2xl bg-raised px-4 py-2.5 text-base text-ink"
            />
            <Pressable onPress={submit} disabled={!text.trim()} className={`h-10 w-10 items-center justify-center rounded-full ${text.trim() ? 'bg-accent' : 'bg-raised'}`} accessibilityLabel="Send message">
              <Send size={17} color={text.trim() ? colors.accentInk : colors.faint} />
            </Pressable>
          </View>
        ) : null}
      </KeyboardAvoidingView>
      <SupportSheet visible={support !== supportSeen} onClose={() => setSupportSeen(support)} />
    </SafeAreaView>
  );
}
