import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { ChevronRight, Database, Eye, Gavel, LifeBuoy, LogOut, ShieldBan, UserPen } from 'lucide-react-native';
import type { ReactNode } from 'react';
import { Alert, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { patch } from '@/api/client';
import { Avatar } from '@/components/Avatar';
import { PersonaName } from '@/components/PersonaBadge';
import { TabHeader } from '@/components/TabHeader';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useMyPersona } from '@/features/me/hooks';
import { setLocale, useT, type Locale } from '@/i18n';
import { useAuth } from '@/lib/auth';
import { setFont, useFontChoice } from '@/lib/fonts';
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
  const { data: p } = useMyPersona();
  const { t, locale } = useT();
  const font = useFontChoice();
  const qc = useQueryClient();
  const lang = useMutation({
    mutationFn: (l: Locale) => patch('/v1/me', { locale: l }),
    onMutate: (l) => setLocale(l),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }),
  });

  return (
    <SafeAreaView edges={['top']} className="flex-1 bg-bg">
      <TabHeader title={t('me.title')} />
      <ScrollView contentContainerClassName="px-5 pb-10 gap-6">
        {p ? (
          <View className="items-center gap-3 rounded-2xl border border-line bg-surface p-6">
            <Avatar seed={p.avatar_seed} size={84} />
            <PersonaName p={p} size="lg" />
            {p.bio ? <Text className="text-center text-muted">{p.bio}</Text> : null}
            <View className="flex-row flex-wrap justify-center gap-2">
              {p.vibes.map((v) => (
                <Chip key={v.slug} label={locale === 'hi' ? v.label_hi : v.label_en} />
              ))}
            </View>
          </View>
        ) : null}

        <View className="overflow-hidden rounded-2xl border border-line bg-surface">
          <SettingsRow icon={<UserPen color={colors.ink} size={20} />} label={t('me.editProfile')} onPress={() => router.push('/settings/profile')} />
          <SettingsRow icon={<ShieldBan color={colors.ink} size={20} />} label={t('me.blocked')} onPress={() => router.push('/settings/blocked')} />
          <SettingsRow icon={<Eye color={colors.ink} size={20} />} label={t('me.revealCard')} onPress={() => router.push('/settings/reveal')} />
          <SettingsRow icon={<LifeBuoy color={colors.mint} size={20} />} label={t('me.safetyCenter')} onPress={() => router.push('/safety')} />
          <SettingsRow icon={<Database color={colors.ink} size={20} />} label={t('me.privacy')} onPress={() => router.push('/settings/privacy')} />
          <SettingsRow icon={<Gavel color={colors.ink} size={20} />} label={t('me.notices')} onPress={() => router.push('/notices')} />
        </View>

        <View className="gap-2">
          <Text variant="label">{t('common.language.title')}</Text>
          <View className="flex-row gap-2">
            {(['en', 'hi'] as const).map((l) => (
              <Chip key={l} label={t(`common.language.${l}`)} selected={locale === l} onPress={() => lang.mutate(l)} />
            ))}
          </View>
        </View>

        <View className="gap-2">
          <Text variant="label">{t('common.font.title')}</Text>
          <View className="flex-row gap-2">
            {(['poppins', 'system'] as const).map((f) => (
              <Chip key={f} label={t(`common.font.${f}`)} selected={font === f} onPress={() => setFont(f)} />
            ))}
          </View>
        </View>

        <View className="overflow-hidden rounded-2xl border border-line bg-surface">
          <SettingsRow
            icon={<LogOut color={colors.danger} size={20} />}
            label={t('me.logout')}
            danger
            onPress={() =>
              Alert.alert(t('me.logoutTitle'), t('me.logoutBody'), [
                { text: t('common.cancel'), style: 'cancel' },
                { text: t('me.logout'), style: 'destructive', onPress: () => logout() },
              ])
            }
          />
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
