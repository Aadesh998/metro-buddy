import { Radio } from 'lucide-react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { colors } from '@/lib/theme';

export default function StudioTab() {
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Vent Studio" />
      <EmptyState icon={<Radio color={colors.accent} size={36} />} title="Live shows" body="Call in to a live audio show and let it out, anonymously." />
    </SafeAreaView>
  );
}
