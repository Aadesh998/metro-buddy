import { Tabs } from 'expo-router/tabs';
import { Coffee, MessageCircle, Radio, TrainFront, UserRound } from 'lucide-react-native';

import { colors } from '@/lib/theme';

export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.faint,
        tabBarStyle: { backgroundColor: colors.surface, borderTopColor: colors.line },
        sceneStyle: { backgroundColor: colors.bg },
      }}
    >
      <Tabs.Screen name="bowls" options={{ title: 'Bowls', tabBarIcon: ({ color, size }) => <Coffee color={color} size={size} /> }} />
      <Tabs.Screen name="chats" options={{ title: 'Chats', tabBarIcon: ({ color, size }) => <MessageCircle color={color} size={size} /> }} />
      <Tabs.Screen name="travel" options={{ title: 'Travel', tabBarIcon: ({ color, size }) => <TrainFront color={color} size={size} /> }} />
      <Tabs.Screen name="studio" options={{ title: 'Studio', tabBarIcon: ({ color, size }) => <Radio color={color} size={size} /> }} />
      <Tabs.Screen name="me" options={{ title: 'Me', tabBarIcon: ({ color, size }) => <UserRound color={color} size={size} /> }} />
    </Tabs>
  );
}
