import { Tabs } from 'expo-router/tabs';
import { Coffee, MessageCircle, Radio, TrainFront, UserRound } from 'lucide-react-native';
import { useEffect } from 'react';

import { useConversations, useRequests } from '@/features/chat/hooks';
import { RealtimeBridge } from '@/features/chat/RealtimeBridge';
import { useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { listenForTaps, registerForPush } from '@/lib/push';
import { colors } from '@/lib/theme';

function useChatBadge() {
  const convs = useConversations();
  const reqs = useRequests('incoming');
  const now = useNow();
  const n = (convs.data ?? []).reduce((a, c) => a + (c.muted_until && Date.parse(c.muted_until) > now ? 0 : c.unread), 0) + (reqs.data?.length ?? 0);
  return n > 0 ? (n > 99 ? '99+' : n) : undefined;
}

// Everything that should run for the whole signed-in session lives here, under the tabs.
export default function TabsLayout() {
  const badge = useChatBadge();
  const { t } = useT();
  useEffect(() => {
    registerForPush(false).catch(() => {});
    return listenForTaps();
  }, []);
  return (
    <>
    <RealtimeBridge />
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.faint,
        tabBarStyle: { backgroundColor: colors.surface, borderTopColor: colors.line },
        sceneStyle: { backgroundColor: colors.bg },
      }}
    >
      <Tabs.Screen name="bowls" options={{ title: t('common.tabs.bowls'), tabBarIcon: ({ color, size }) => <Coffee color={color} size={size} /> }} />
      <Tabs.Screen
        name="chats"
        options={{ title: t('common.tabs.chats'), tabBarBadge: badge, tabBarBadgeStyle: { backgroundColor: colors.accent, color: colors.accentInk }, tabBarIcon: ({ color, size }) => <MessageCircle color={color} size={size} /> }}
      />
      <Tabs.Screen name="travel" options={{ title: t('common.tabs.travel'), tabBarIcon: ({ color, size }) => <TrainFront color={color} size={size} /> }} />
      <Tabs.Screen name="studio" options={{ title: t('common.tabs.studio'), tabBarIcon: ({ color, size }) => <Radio color={color} size={size} /> }} />
      <Tabs.Screen name="me" options={{ title: t('common.tabs.me'), tabBarIcon: ({ color, size }) => <UserRound color={color} size={size} /> }} />
    </Tabs>
    </>
  );
}
