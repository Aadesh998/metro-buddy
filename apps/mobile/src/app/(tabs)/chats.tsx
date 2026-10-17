import { MessageCircle } from 'lucide-react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState } from '@/components/EmptyState';
import { TabHeader } from '@/components/TabHeader';
import { colors } from '@/lib/theme';

export default function ChatsTab() {
  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Chats" />
      <EmptyState icon={<MessageCircle color={colors.accent} size={36} />} title="No chats yet" body="Chats start when someone accepts your request." />
    </SafeAreaView>
  );
}
