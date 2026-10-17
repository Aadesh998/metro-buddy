import { Text } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

export default function Home() {
  return (
    <SafeAreaView className="flex-1 items-center justify-center bg-white dark:bg-neutral-950">
      <Text className="text-2xl font-semibold text-neutral-900 dark:text-neutral-100">
        Metro Buddy
      </Text>
    </SafeAreaView>
  );
}
