import { Coffee } from 'lucide-react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { colors } from '@/lib/theme';

export default function BowlsTab() {
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Bowls" />
      <EmptyState icon={<Coffee color={colors.accent} size={36} />} title="Bowls are brewing" body="Communities for your line, station and office hub." />
    </SafeAreaView>
  );
}
