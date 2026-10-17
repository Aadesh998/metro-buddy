import { Eye, Flag, Lock, MapPin, ShieldCheck, TrainFront } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { View } from 'react-native';

import { Helplines } from '@/components/Helplines';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { colors } from '@/lib/theme';

function Tip({ icon, title, body }: { icon: ReactNode; title: string; body: string }) {
  return (
    <View className="flex-row gap-3 rounded-2xl border border-line bg-surface p-4">
      <View className="pt-0.5">{icon}</View>
      <View className="flex-1 gap-1">
        <Text variant="label">{title}</Text>
        <Text variant="caption" className="leading-5">
          {body}
        </Text>
      </View>
    </View>
  );
}

export default function SafetyCenter() {
  return (
    <Screen scroll edges={['top', 'bottom']} className="gap-4">
      <ScreenHeader title="Safety Center" />
      <Text className="text-muted">Metro Buddy is built so you stay anonymous until you choose otherwise. If you ever feel unsafe, these help.</Text>

      <Text variant="heading" className="mt-2">
        Helplines
      </Text>
      <Helplines />

      <Text variant="heading" className="mt-4">
        Staying safe
      </Text>
      <Tip icon={<Lock color={colors.accent} size={20} />} title="Your identity stays hidden" body="Phone numbers, emails, handles and links are hidden automatically in posts and chats. Only reveal more when you both choose to." />
      <Tip icon={<MapPin color={colors.accent} size={20} />} title="Meet only inside stations" body="Travel buddies always meet on a platform, at a coach. Never share your home or office address." />
      <Tip icon={<TrainFront color={colors.accent} size={20} />} title="Share your trip" body="Every travel match has a Share Trip button and a one-tap SOS that alerts your trusted contacts." />
      <Tip icon={<Eye color={colors.accent} size={20} />} title="Trust your gut" body="If something feels off, leave the chat. You never owe anyone a reply." />
      <Tip icon={<Flag color={colors.accent} size={20} />} title="Report, block, mute" body="Tap ⋯ on any profile, post, comment or message. Reports are anonymous and reviewed by people, not bots." />
      <Tip icon={<ShieldCheck color={colors.accent} size={20} />} title="Grievance officer" body="Unhappy with a decision? Appeal from the notice you received, or write to our grievance officer from the website. We acknowledge within 24 hours." />
    </Screen>
  );
}
