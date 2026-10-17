import { Eye, Flag, Lock, MapPin, ShieldCheck, TrainFront } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { View } from 'react-native';

import { Helplines } from '@/components/Helplines';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
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
  const { t } = useT();
  return (
    <Screen scroll edges={['top', 'bottom']} className="gap-4">
      <ScreenHeader title={t('safety.center.title')} />
      <Text className="text-muted">{t('safety.center.intro')}</Text>

      <Text variant="heading" className="mt-2">
        {t('safety.center.helplines')}
      </Text>
      <Helplines />

      <Text variant="heading" className="mt-4">
        {t('safety.center.stayingSafe')}
      </Text>
      <Tip icon={<Lock color={colors.accent} size={20} />} title={t('safety.center.identityTitle')} body={t('safety.center.identityBody')} />
      <Tip icon={<MapPin color={colors.accent} size={20} />} title={t('safety.center.meetTitle')} body={t('safety.center.meetBody')} />
      <Tip icon={<TrainFront color={colors.accent} size={20} />} title={t('safety.center.shareTitle')} body={t('safety.center.shareBody')} />
      <Tip icon={<Eye color={colors.accent} size={20} />} title={t('safety.center.gutTitle')} body={t('safety.center.gutBody')} />
      <Tip icon={<Flag color={colors.accent} size={20} />} title={t('safety.center.reportTitle')} body={t('safety.center.reportBody')} />
      <Tip icon={<ShieldCheck color={colors.accent} size={20} />} title={t('safety.center.grievanceTitle')} body={t('safety.center.grievanceBody')} />
    </Screen>
  );
}
