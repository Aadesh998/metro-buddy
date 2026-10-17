import { FlashList, type FlashListRef } from '@shopify/flash-list';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import * as Clipboard from 'expo-clipboard';
import * as ImagePicker from 'expo-image-picker';
import { router, useLocalSearchParams } from 'expo-router';
import { ArrowDown, Check, ChevronLeft, ImagePlus, Keyboard as KeyboardIcon, Pencil, Smile, X, Info, MoreHorizontal, Send } from 'lucide-react-native';
import { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, Keyboard, Pressable, View } from 'react-native';
import { KeyboardAvoidingView } from 'react-native-keyboard-controller';
import { SafeAreaView } from 'react-native-safe-area-context';

import { FontTextInput } from '@/components/ui/FontTextInput';
import { del, post } from '@/api/client';
import type { Conversation, Message } from '@/api/types';
import { SupportSheet } from '@/components/SupportSheet';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { useConversationTitle } from '@/features/chat/ConversationRow';
import { ExpressionPanel } from '@/features/chat/ExpressionPanel';
import { chatKeys, useConversation, useEditMessage, useMessages, useOpenViewOnce, useSend, useSendImage, useSendSticker } from '@/features/chat/hooks';
import { MessageBubble } from '@/features/chat/MessageBubble';
import { ViewOnceViewer } from '@/features/chat/ViewOnceViewer';
import { activeConversation, useOnline, useReceipts, useSupportSignal, useTyping } from '@/features/chat/store';
import { useSafetySheet } from '@/features/safety/SafetySheet';
import { useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { send as frame } from '@/lib/realtime';
import { colors } from '@/lib/theme';
import { until } from '@/lib/time';

function Header({ c, id }: { c?: Conversation; id: string }) {
  const { t, tp } = useT();
  const { title, other } = useConversationTitle(c ?? ({ members: [], kind: 'group', title: '' } as unknown as Conversation));
  const online = useOnline(other?.id);
  const typing = useTyping(id);
  const safety = useSafetySheet();
  let sub = '';
  if (typing.length) sub = c?.kind === 'dm' ? t('chat.typing') : t('chat.typingMany', { n: typing.length });
  else if (c?.kind === 'discussion' && c.ends_at) sub = c.closed ? t('chat.roomClosed') : t('chat.live', { left: until(c.ends_at) });
  else if (c && c.kind !== 'dm' && c.kind !== 'travel') sub = tp('chat.members', c.member_count);
  else if (online) sub = t('chat.online');
  return (
    <View className="h-14 flex-row items-center border-b border-line px-2">
      <Pressable onPress={() => router.back()} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel={t('common.back')}>
        <ChevronLeft size={26} color={colors.ink} />
      </Pressable>
      <View className="flex-1">
        <Text className="font-semibold" numberOfLines={1}>
          {c ? title : ''}
        </Text>
        {sub ? <Text className={`text-xs ${typing.length ? 'text-brand' : 'text-muted'}`}>{sub}</Text> : null}
      </View>
      {c && (c.kind === 'dm' || c.kind === 'travel') && other ? (
        <Pressable onPress={() => safety.open({ type: 'conversation', id, personaId: other.id, alias: other.alias })} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel={t('common.safetyOptions')}>
          <MoreHorizontal size={22} color={colors.ink} />
        </Pressable>
      ) : null}
      {c ? (
        <Pressable onPress={() => router.push(`/chat/${id}/info`)} hitSlop={10} className="h-10 w-10 items-center justify-center" accessibilityLabel={t('chat.chatInfo')}>
          <Info size={20} color={colors.ink} />
        </Pressable>
      ) : null}
    </View>
  );
}

export default function ChatScreen() {
  const { t, tp } = useT();
  const { id } = useLocalSearchParams<{ id: string }>();
  const qc = useQueryClient();
  const conv = useConversation(id);
  const msgs = useMessages(id);
  const send = useSend(id);
  const sendImage = useSendImage(id);
  const sendSticker = useSendSticker(id);
  const editMessage = useEditMessage();
  const openViewOnce = useOpenViewOnce();
  const [editing, setEditing] = useState<Message | null>(null);
  const [panel, setPanel] = useState(false);
  const [viewUrl, setViewUrl] = useState<string | null>(null);
  const safety = useSafetySheet();
  const receipts = useReceipts(id);
  const support = useSupportSignal();
  const [supportSeen, setSupportSeen] = useState(support);
  const [text, setText] = useState('');
  const lastTyping = useRef(0);
  const now = useNow(15_000);
  const items = useMemo(() => (msgs.data?.pages.flatMap((p) => p.items) ?? []).filter((m) => !m.expires_at || Date.parse(m.expires_at) > now), [msgs.data, now]);
  // The cache keeps newest first; the list renders oldest first and stays pinned to the bottom.
  const ordered = useMemo(() => [...items].reverse(), [items]);
  const listRef = useRef<FlashListRef<Message>>(null);
  const nearBottom = useRef(true);
  const [unseen, setUnseen] = useState(0);
  const newestId = items[0]?.id;
  const lastNewest = useRef(newestId);
  useEffect(() => {
    if (!newestId || newestId === lastNewest.current) return;
    const mine = items[0]?.is_mine;
    lastNewest.current = newestId;
    if (mine || nearBottom.current) requestAnimationFrame(() => listRef.current?.scrollToEnd({ animated: true }));
    else setUnseen((n) => n + 1);
  }, [newestId, items]);
  useEffect(() => {
    const sub = Keyboard.addListener('keyboardDidShow', () => {
      if (nearBottom.current) listRef.current?.scrollToEnd({ animated: true });
    });
    return () => sub.remove();
  }, []);
  function jumpToLatest() {
    setUnseen(0);
    listRef.current?.scrollToEnd({ animated: true });
  }
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
    if (m.body && !m.deleted && m.kind === 'text') actions.push({ text: t('common.copy'), onPress: () => Clipboard.setStringAsync(m.body) });
    if (m.is_mine && !m.local && !m.deleted && m.kind === 'text' && Date.now() - Date.parse(m.created_at) < 15 * 60_000) {
      actions.push({
        text: t('chat.edit'),
        onPress: () => {
          setEditing(m);
          setText(m.body);
        },
      });
    }
    if (m.is_mine && !m.local) actions.push({ text: t('chat.deleteForEveryone'), style: 'destructive', onPress: () => del(`/v1/messages/${m.id}`) });
    if (!m.is_mine && m.sender) actions.push({ text: t('chat.reportOrBlock'), onPress: () => safety.open({ type: 'message', id: m.id, personaId: m.sender!.id, alias: m.sender!.alias }) });
    actions.push({ text: t('common.cancel'), style: 'cancel' });
    Alert.alert(t('chat.message'), undefined, actions);
  }

  function onChange(v: string) {
    setText(v.slice(0, 4000));
    if (Date.now() - lastTyping.current > 2500) {
      lastTyping.current = Date.now();
      frame('typing', { conversation_id: id });
    }
  }

  async function pickImage() {
    const r = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ['images'], quality: 0.85 });
    if (r.canceled) return;
    const a = r.assets[0];
    const go = (viewOnce: boolean) =>
      sendImage.mutate(
        { asset: { uri: a.uri, mimeType: a.mimeType, fileSize: a.fileSize }, viewOnce },
        { onSuccess: (m) => m.media[0]?.pending && toast(t('chat.photoPending'), 'success'), onError: (e) => toast(e.message) },
      );
    Alert.alert(t('chat.photoSheetTitle'), t('chat.viewOnceNote'), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('chat.sendViewOnce'), onPress: () => go(true) },
      { text: t('chat.sendNormally'), onPress: () => go(false) },
    ]);
  }

  function openPhoto(m: Message) {
    openViewOnce.mutate(m, { onSuccess: (r) => setViewUrl(r.url), onError: () => toast(t('chat.photoGone')) });
  }

  async function submit() {
    const body = text.trim();
    if (!body) return;
    setText('');
    if (editing) {
      const m = editing;
      setEditing(null);
      if (body !== m.body) editMessage.mutate({ id: m.id, body }, { onError: (e) => toast(e.message) });
      return;
    }
    await send(body);
  }

  const notJoined = conv.isError && !c;
  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <Header c={c} id={id} />
      <KeyboardAvoidingView className="flex-1" behavior="padding" keyboardVerticalOffset={0}>
        {notJoined ? (
          <View className="flex-1 items-center justify-center gap-3 px-8">
            <Text className="text-center text-muted">{t('chat.notInConversation')}</Text>
            <Button title={t('chat.joinRoom')} onPress={() => join.mutate()} loading={join.isPending} />
          </View>
        ) : (
          <View className="flex-1">
            <FlashList
              ref={listRef}
              data={ordered}
              keyExtractor={(m) => m.id}
              maintainVisibleContentPosition={{ startRenderingFromBottom: true, autoscrollToBottomThreshold: 0.15, animateAutoScrollToBottom: true }}
              renderItem={({ item, index }) => (
                <MessageBubble
                  m={item}
                  showSender={isGroup && ordered[index - 1]?.sender?.id !== item.sender?.id}
                  seen={item.id === seenId}
                  onLongPress={onLongPress}
                  onRetry={(m) => send(m.body)}
                  onPickOpener={setText}
                  onOpenViewOnce={openPhoto}
                />
              )}
              onStartReached={() => msgs.hasNextPage && !msgs.isFetchingNextPage && msgs.fetchNextPage()}
              onStartReachedThreshold={0.3}
              onScroll={(e) => {
                const { contentOffset, contentSize, layoutMeasurement } = e.nativeEvent;
                const atBottom = contentSize.height - contentOffset.y - layoutMeasurement.height < 120;
                nearBottom.current = atBottom;
                if (atBottom && unseen) setUnseen(0);
              }}
              scrollEventThrottle={100}
              keyboardDismissMode="interactive"
              keyboardShouldPersistTaps="handled"
              contentContainerStyle={{ paddingVertical: 8 }}
            />
            {unseen > 0 ? (
              <Pressable
                onPress={jumpToLatest}
                accessibilityRole="button"
                className="absolute bottom-3 self-center flex-row items-center gap-1.5 rounded-full bg-accent px-4 py-2 shadow"
              >
                <ArrowDown size={14} color={colors.accentInk} />
                <Text className="text-sm font-semibold text-accent-ink">{tp('chat.newMessages', unseen)}</Text>
              </Pressable>
            ) : null}
          </View>
        )}
        {c?.closed ? (
          <View className="border-t border-line p-4">
            <Text className="text-center text-muted">{t('chat.conversationClosed')}</Text>
          </View>
        ) : c ? (
          <View>
            {editing ? (
              <View className="flex-row items-center gap-2 border-t border-line bg-surface px-4 pt-2">
                <Pencil size={14} color={colors.brand} />
                <Text className="flex-1 text-xs text-brand">{t('chat.editing')}</Text>
                <Pressable
                  onPress={() => {
                    setEditing(null);
                    setText('');
                  }}
                  hitSlop={10}
                  accessibilityLabel={t('common.cancel')}
                >
                  <X size={16} color={colors.muted} />
                </Pressable>
              </View>
            ) : null}
            <View className={`flex-row items-end gap-1.5 bg-surface px-2 py-2 ${editing ? '' : 'border-t border-line'}`}>
              {c.kind !== 'discussion' && !editing ? (
                <Pressable onPress={pickImage} disabled={sendImage.isPending} className="h-10 w-9 items-center justify-center" accessibilityLabel={t('chat.sendPhoto')}>
                  {sendImage.isPending ? <ActivityIndicator color={colors.brand} /> : <ImagePlus size={20} color={colors.muted} />}
                </Pressable>
              ) : null}
              <Pressable
                onPress={() => {
                  if (panel) {
                    setPanel(false);
                  } else {
                    Keyboard.dismiss();
                    setPanel(true);
                  }
                }}
                className="h-10 w-9 items-center justify-center"
                accessibilityLabel={t('chat.emoji')}
              >
                {panel ? <KeyboardIcon size={20} color={colors.brand} /> : <Smile size={20} color={colors.muted} />}
              </Pressable>
              <FontTextInput
                value={text}
                onChangeText={onChange}
                onFocus={() => setPanel(false)}
                placeholder={c.slow_mode_sec ? t('chat.slowModePlaceholder', { n: c.slow_mode_sec }) : t('chat.messagePlaceholder')}
                placeholderTextColor={colors.faint}
                multiline
                className="max-h-32 min-h-10 flex-1 rounded-2xl bg-raised px-4 py-2.5 text-base text-ink"
              />
              <Pressable onPress={submit} disabled={!text.trim()} className={`h-10 w-10 items-center justify-center rounded-full ${text.trim() ? 'bg-accent' : 'bg-raised'}`} accessibilityLabel={t('chat.sendMessage')}>
                {editing ? <Check size={18} color={text.trim() ? colors.accentInk : colors.faint} /> : <Send size={17} color={text.trim() ? colors.accentInk : colors.faint} />}
              </Pressable>
            </View>
            {panel ? (
              <ExpressionPanel
                onEmoji={(e) => setText((v) => (v + e).slice(0, 4000))}
                onSticker={(sid) => {
                  setPanel(false);
                  sendSticker.mutate(sid, { onError: (e) => toast(e.message) });
                }}
              />
            ) : null}
          </View>
        ) : null}
      </KeyboardAvoidingView>
      <SupportSheet visible={support !== supportSeen} onClose={() => setSupportSeen(support)} />
      <ViewOnceViewer url={viewUrl} onClose={() => setViewUrl(null)} />
    </SafeAreaView>
  );
}
