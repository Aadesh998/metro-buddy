import { TrainFront } from 'lucide-react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { colors } from '@/lib/theme';

export default function TravelTab() {
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Travel" />
      <EmptyState icon={<TrainFront color={colors.accent} size={36} />} title="Travel buddies" body="Find someone on your route at your time." />
    </SafeAreaView>
  );
}
