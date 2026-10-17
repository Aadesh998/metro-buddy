import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Helplines } from '@/components/Helplines';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';

// Shown to the writer only, when what they wrote suggests they may be struggling. Never punitive.
export function SupportSheet({ visible, onClose }: { visible: boolean; onClose: () => void }) {
  const insets = useSafeAreaInsets();
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <Pressable className="flex-1 bg-black/60" onPress={onClose} />
      <View className="rounded-t-2xl border-t border-line bg-surface px-5 pt-5" style={{ paddingBottom: insets.bottom + 16 }}>
        <Text variant="heading" className="mb-1">
          You don’t have to carry this alone
        </Text>
        <Text className="mb-4 text-muted">Your post is up. If things feel heavy right now, these people are there for you, free and confidential, any time.</Text>
        <Helplines only={['14416', '112']} />
        <Button title="Thanks" kind="secondary" className="mt-4" onPress={onClose} />
      </View>
    </Modal>
  );
}
