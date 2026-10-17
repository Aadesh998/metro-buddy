import { router } from 'expo-router';
import { Plus } from 'lucide-react-native';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ScreenHeader } from '@/components/ScreenHeader';
import { Text } from '@/components/ui/Text';
import { EpisodeCard } from '@/features/studio/EpisodeCard';
import { useShows } from '@/features/studio/hooks';
import { useNow } from '@/lib/clock';
import { colors } from '@/lib/theme';

export default function MyShows() {
  const shows = useShows('mine');
  const now = useNow(60_000);
  return (
    <SafeAreaView className="flex-1 bg-bg">
      <ScreenHeader
        title="My shows"
        right={
          <Pressable onPress={() => router.push('/studio/new-show')} hitSlop={12} accessibilityLabel="New show">
            <Plus color={colors.ink} size={22} />
          </Pressable>
        }
      />
      <ScrollView contentContainerClassName="gap-4 p-4">
        {shows.data?.length === 0 ? <Text className="text-center text-muted">You haven’t made a show yet.</Text> : null}
        {(shows.data ?? []).map((s) => (
          <View key={s.id} className="gap-2">
            <Pressable onPress={() => router.push(`/show/${s.id}`)} className="flex-row items-center justify-between">
              <Text variant="heading">{s.title}</Text>
              <Text variant="caption">{s.follower_count} following</Text>
            </Pressable>
            {s.next_episode ? <EpisodeCard ep={s.next_episode} now={now} /> : <Text variant="caption">No episode scheduled.</Text>}
          </View>
        ))}
      </ScrollView>
    </SafeAreaView>
  );
}
