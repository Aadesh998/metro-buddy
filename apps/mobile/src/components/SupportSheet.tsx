import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Helplines } from '@/components/Helplines';
import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';

// Shown to the writer only, when what they wrote suggests they may be struggling. Never punitive.
export function SupportSheet({ visible, onClose }: { visible: boolean; onClose: () => void }) {
  const insets = useSafeAreaInsets();
  const { t } = useT();
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <Pressable className="flex-1 bg-black/60" onPress={onClose} />
      <View className="rounded-t-2xl border-t border-line bg-surface px-5 pt-5" style={{ paddingBottom: insets.bottom + 16 }}>
        <Text variant="heading" className="mb-1">
          {t('safety.support.title')}
        </Text>
        <Text className="mb-4 text-muted">{t('safety.support.body')}</Text>
        <Helplines only={['14416', '112']} />
        <Button title={t('safety.support.thanks')} kind="secondary" className="mt-4" onPress={onClose} />
      </View>
    </Modal>
  );
}
