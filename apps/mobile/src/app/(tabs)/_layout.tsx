import { Tabs } from 'expo-router/tabs';
import { Coffee, MessageCircle, Radio, TrainFront, UserRound, Gamepad2 } from 'lucide-react-native';
import { useEffect } from 'react';
import { Text as RNText } from 'react-native';

import { useConversations, useRequests } from '@/features/chat/hooks';
import { RealtimeBridge } from '@/features/chat/RealtimeBridge';
import { useT } from '@/i18n';
import { useNow } from '@/lib/clock';
import { fontFamily, useFontChoice } from '@/lib/fonts';
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
  const font = useFontChoice();
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
        tabBarActiveTintColor: colors.ink,
        tabBarInactiveTintColor: colors.faint,
        tabBarStyle: { backgroundColor: colors.surface, borderTopColor: colors.line },
        // Six tabs on a 360dp phone: long labels (German, Russian) shrink to fit instead of being cut.
        tabBarLabel: ({ children, color }) => (
          <RNText
            numberOfLines={1}
            adjustsFontSizeToFit
            minimumFontScale={0.7}
            style={[{ color, fontSize: 10, paddingHorizontal: 2 }, font === 'system' ? null : { fontFamily: fontFamily(font, 'medium') }]}
          >
            {children}
          </RNText>
        ),
        sceneStyle: { backgroundColor: colors.bg },
      }}
    >
      <Tabs.Screen name="bowls" options={{ title: t('common.tabs.bowls'), tabBarIcon: ({ color, size }) => <Coffee color={color} size={size} /> }} />
      <Tabs.Screen
        name="chats"
        options={{ title: t('common.tabs.chats'), tabBarBadge: badge, tabBarBadgeStyle: { backgroundColor: colors.accent, color: colors.accentInk }, tabBarIcon: ({ color, size }) => <MessageCircle color={color} size={size} /> }}
      />
      <Tabs.Screen name="games" options={{ title: t('games.tab'), tabBarIcon: ({ color, size }) => <Gamepad2 color={color} size={size} /> }} />
      <Tabs.Screen name="travel" options={{ title: t('common.tabs.travel'), tabBarIcon: ({ color, size }) => <TrainFront color={color} size={size} /> }} />
      <Tabs.Screen name="studio" options={{ title: t('common.tabs.studio'), tabBarIcon: ({ color, size }) => <Radio color={color} size={size} /> }} />
      <Tabs.Screen name="me" options={{ title: t('common.tabs.me'), tabBarIcon: ({ color, size }) => <UserRound color={color} size={size} /> }} />
    </Tabs>
    </>
  );
}
