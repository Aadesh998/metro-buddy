import { Image } from 'expo-image';
import { X } from 'lucide-react-native';
import { Modal, Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';

// Shows a view-once photo from a one-minute link. Nothing is cached, so closing it is final.
export function ViewOnceViewer({ url, onClose }: { url: string | null; onClose: () => void }) {
  const { t } = useT();
  const insets = useSafeAreaInsets();
  return (
    <Modal visible={!!url} animationType="fade" onRequestClose={onClose} statusBarTranslucent>
      <View className="flex-1 bg-black">
        {url ? <Image source={{ uri: url }} cachePolicy="none" contentFit="contain" style={{ flex: 1 }} /> : null}
        <Pressable onPress={onClose} accessibilityLabel={t('common.close')} className="absolute right-4 h-10 w-10 items-center justify-center rounded-full bg-black/60" style={{ top: insets.top + 8 }}>
          <X size={22} color="#fff" />
        </Pressable>
        <Text className="absolute self-center text-xs text-white/70" style={{ bottom: insets.bottom + 16 }}>
          {t('chat.viewOnceNote')}
        </Text>
      </View>
    </Modal>
  );
}
