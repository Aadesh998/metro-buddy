import { FlashList } from '@shopify/flash-list';
import { Search, X } from 'lucide-react-native';
import { useMemo, useState } from 'react';
import { Modal, Pressable, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Station } from '@/api/types';
import { Text } from '@/components/ui/Text';
import { searchStations, useStations } from '@/features/metro/hooks';
import { colors } from '@/lib/theme';

export function LineDots({ station }: { station: Station }) {
  return (
    <View className="flex-row gap-1">
      {station.lines.map((l) => (
        <View key={l.id} style={{ backgroundColor: l.color }} className="h-2.5 w-2.5 rounded-full" />
      ))}
    </View>
  );
}

export function StationPicker({
  cityId,
  visible,
  title,
  onClose,
  onPick,
}: {
  cityId: string;
  visible: boolean;
  title: string;
  onClose: () => void;
  onPick: (s: Station) => void;
}) {
  const { data = [], isPending } = useStations(cityId);
  const [q, setQ] = useState('');
  const results = useMemo(() => searchStations(data, q), [data, q]);

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <SafeAreaView className="flex-1 bg-bg" edges={['top', 'bottom']}>
        <View className="flex-row items-center justify-between px-5 pb-3 pt-2">
          <Text variant="heading">{title}</Text>
          <Pressable onPress={onClose} accessibilityLabel="Close" hitSlop={12}>
            <X color={colors.muted} size={22} />
          </Pressable>
        </View>
        <View className="mx-5 mb-2 h-12 flex-row items-center gap-2 rounded-xl border border-line bg-surface px-3">
          <Search color={colors.faint} size={18} />
          <TextInput
            value={q}
            onChangeText={setQ}
            placeholder="Search stations"
            placeholderTextColor={colors.faint}
            className="flex-1 text-base text-ink"
            autoFocus
            autoCorrect={false}
          />
        </View>
        {isPending ? (
          <Text className="px-5 py-6 text-muted">Loading stations…</Text>
        ) : (
          <FlashList
            data={results}
            keyExtractor={(s) => s.id}
            keyboardShouldPersistTaps="handled"
            ListEmptyComponent={<Text className="px-5 py-6 text-muted">No station matches “{q}”.</Text>}
            renderItem={({ item }) => (
              <Pressable
                onPress={() => {
                  onPick(item);
                  setQ('');
                }}
                className="flex-row items-center justify-between border-b border-line/60 px-5 py-3.5 active:bg-surface"
              >
                <View className="flex-1 gap-1">
                  <Text>{item.name}</Text>
                  <View className="flex-row flex-wrap gap-1.5">
                    {item.lines.map((l) => (
                      <View key={l.id} className="flex-row items-center gap-1 rounded-full bg-raised px-2 py-0.5">
                        <View style={{ backgroundColor: l.color }} className="h-2 w-2 rounded-full" />
                        <Text className="text-[11px] text-muted">{l.name}</Text>
                      </View>
                    ))}
                  </View>
                </View>
                {item.is_interchange ? <Text variant="caption">Interchange</Text> : null}
              </Pressable>
            )}
          />
        )}
      </SafeAreaView>
    </Modal>
  );
}
