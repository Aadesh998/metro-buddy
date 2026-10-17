import { BellRing, X } from 'lucide-react-native';
import { useEffect, useState } from 'react';
import { Pressable, View } from 'react-native';

import { Button } from '@/components/ui/Button';
import { Text } from '@/components/ui/Text';
import { pushStatus, registerForPush, wasAsked } from '@/lib/push';
import { colors } from '@/lib/theme';

// A soft ask before the OS prompt: the system dialog can only be shown once.
export function PushPrompt() {
  const [show, setShow] = useState(false);
  useEffect(() => {
    (async () => setShow((await pushStatus()) === 'undetermined' && !(await wasAsked())))();
  }, []);
  if (!show) return null;
  return (
    <View className="mx-4 mb-3 flex-row gap-3 rounded-2xl border border-accent/40 bg-accent/5 p-4">
      <BellRing size={20} color={colors.accent} />
      <View className="flex-1 gap-2">
        <Text variant="label">Know when someone replies</Text>
        <Text variant="caption">Message requests, replies and travel matches. Never message contents on your lock screen. Quiet hours in Settings.</Text>
        <Button title="Turn on notifications" className="h-9 self-start px-4" onPress={() => registerForPush(true).finally(() => setShow(false))} />
      </View>
      <Pressable onPress={() => setShow(false)} hitSlop={10} accessibilityLabel="Not now">
        <X size={16} color={colors.muted} />
      </Pressable>
    </View>
  );
}
