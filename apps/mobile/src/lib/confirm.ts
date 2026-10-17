import { Alert, Platform } from 'react-native';

import { t } from '@/i18n';

// Alert.alert is a no-op on web, so web falls back to the browser dialog.
export function confirm(title: string, message: string, ok: string, onOk: () => void) {
  if (Platform.OS === 'web') {
    if (window.confirm(`${title}\n\n${message}`)) onOk();
    return;
  }
  Alert.alert(title, message, [
    { text: t('common.cancel'), style: 'cancel' },
    { text: ok, style: 'destructive', onPress: onOk },
  ]);
}
