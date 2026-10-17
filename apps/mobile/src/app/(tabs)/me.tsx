import { router } from 'expo-router';
import { ChevronRight, Database, Gavel, LifeBuoy, LogOut, ShieldBan, UserPen } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { Alert, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { TabHeader } from '@/components/TabHeader';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useMe, useMyPersona } from '@/features/me/hooks';
import { useAuth } from '@/lib/auth';
import { colors } from '@/lib/theme';

export function SettingsRow({ icon, label, onPress, danger }: { icon: ReactNode; label: string; onPress: () => void; danger?: boolean }) {
  return (
    <Pressable onPress={onPress} className="flex-row items-center gap-4 px-4 py-4 active:bg-raised" accessibilityRole="button">
      {icon}
      <Text className={`flex-1 ${danger ? 'text-danger' : 'text-ink'}`}>{label}</Text>
      {!danger ? <ChevronRight color={colors.faint} size={18} /> : null}
    </Pressable>
  );
}

export default function MeTab() {
  const { logout } = useAuth();
  const { data: me } = useMe();
  const { data: p } = useMyPersona();

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title="Me" />
      <ScrollView contentContainerClassName="px-5 pb-10 gap-6">
        {p ? (
          <View className="items-center gap-3 rounded-2xl border border-line bg-surface p-6">
            <Avatar seed={p.avatar_seed} size={84} />
            <PersonaName p={p} size="lg" />
            {p.bio ? <Text className="text-center text-muted">{p.bio}</Text> : null}
            <View className="flex-row flex-wrap justify-center gap-2">
              {p.vibes.map((v) => (
                <Chip key={v.slug} label={me?.locale === 'hi' ? v.label_hi : v.label_en} />
              ))}
            </View>
          </View>
        ) : null}

        <View className="overflow-hidden rounded-2xl border border-line bg-surface">
          <SettingsRow icon={<UserPen color={colors.ink} size={20} />} label="Edit profile" onPress={() => router.push('/settings/profile')} />
          <SettingsRow icon={<ShieldBan color={colors.ink} size={20} />} label="Blocked & muted" onPress={() => router.push('/settings/blocked')} />
          <SettingsRow icon={<LifeBuoy color={colors.mint} size={20} />} label="Safety Center" onPress={() => router.push('/safety')} />
          <SettingsRow icon={<Database color={colors.ink} size={20} />} label="Privacy & your data" onPress={() => router.push('/settings/privacy')} />
          <SettingsRow icon={<Gavel color={colors.ink} size={20} />} label="Account notices & appeals" onPress={() => router.push('/notices')} />
        </View>

        <View className="overflow-hidden rounded-2xl border border-line bg-surface">
          <SettingsRow
            icon={<LogOut color={colors.danger} size={20} />}
            label="Log out"
            danger
            onPress={() =>
              Alert.alert('Log out?', 'You can log back in with your phone number.', [
                { text: 'Cancel', style: 'cancel' },
                { text: 'Log out', style: 'destructive', onPress: () => logout() },
              ])
            }
          />
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
