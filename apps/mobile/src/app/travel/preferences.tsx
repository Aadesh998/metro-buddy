import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { ChevronRight, Users } from 'lucide-react-native';
import { Pressable, Switch, View } from 'react-native';

import { get, patch } from '@/api/client';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Chip } from '@/components/ui/Chip';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { useMe } from '@/features/me/hooks';
import { useT, type Key } from '@/i18n';
import { colors } from '@/lib/theme';

type Settings = { match_same_gender_only: boolean; match_verified_only: boolean; match_age_min: number | null; match_age_max: number | null };

const BANDS: { label: string; min: number | null; max: number | null }[] = [
  { label: 'Any age', min: null, max: null },
  { label: '18–25', min: 18, max: 25 },
  { label: '22–30', min: 22, max: 30 },
  { label: '28–40', min: 28, max: 40 },
  { label: '35+', min: 35, max: 80 },
];

const GENDERS: { key: string; label: Key }[] = [
  { key: 'woman', label: 'travel.prefs.woman' },
  { key: 'man', label: 'travel.prefs.man' },
  { key: 'non_binary', label: 'travel.prefs.nonBinary' },
  { key: 'prefer_not_to_say', label: 'travel.prefs.preferNot' },
];

const ROWS: { key: string; label: Key; body: Key }[] = [
  { key: 'match_same_gender_only', label: 'travel.prefs.sameGender', body: 'travel.prefs.sameGenderBody' },
  { key: 'match_verified_only', label: 'travel.prefs.verified', body: 'travel.prefs.verifiedBody' },
];

export default function Preferences() {
  const { t } = useT();
  const qc = useQueryClient();
  const me = useMe().data;
  const q = useQuery({ queryKey: ['settings'], queryFn: () => get<Settings>('/v1/me/settings') });
  const save = useMutation({ mutationFn: (b: object) => patch<Settings>('/v1/me/settings', b), onSuccess: (s) => qc.setQueryData(['settings'], s) });
  const setGender = useMutation({ mutationFn: (g: string) => patch('/v1/me', { gender: g }), onSuccess: () => qc.invalidateQueries({ queryKey: ['me'] }) });
  const s = q.data;
  if (!s) return null;
  const band = BANDS.find((b) => b.min === s.match_age_min && b.max === s.match_age_max) ?? BANDS[0];

  return (
    <Screen scroll className="gap-5">
      <ScreenHeader title={t('travel.prefs.title')} />
      <Text className="text-muted">{t('travel.prefs.intro')}</Text>
      <View className="gap-2">
        <Text variant="label">{t('travel.prefs.gender')}</Text>
        <View className="flex-row flex-wrap gap-2">
          {GENDERS.map((g) => (
            <Chip key={g.key} label={t(g.label)} selected={me?.gender === g.key} onPress={() => setGender.mutate(g.key)} />
          ))}
        </View>
      </View>
      {ROWS.map((row) => (
        <View key={row.key} className="flex-row items-center gap-4 rounded-2xl border border-line bg-surface p-4">
          <View className="flex-1 gap-1">
            <Text variant="label">{t(row.label)}</Text>
            <Text variant="caption">{t(row.body)}</Text>
          </View>
          <Switch
            value={!!s[row.key as keyof Settings]}
            onValueChange={(v) => save.mutate({ [row.key]: v })}
            trackColor={{ true: colors.brand, false: colors.line }}
            thumbColor={colors.surface}
          />
        </View>
      ))}
      <View className="gap-2">
        <Text variant="label">{t('travel.prefs.ageRange')}</Text>
        <View className="flex-row flex-wrap gap-2">
          {BANDS.map((b) => (
            <Chip key={b.label} label={b.min ? b.label : t('travel.prefs.anyAge')} selected={b === band} onPress={() => save.mutate(b.min ? { match_age_min: b.min, match_age_max: b.max } : { clear_age_band: true })} />
          ))}
        </View>
      </View>
      <Pressable onPress={() => router.push('/settings/contacts')} className="flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-4">
        <Users size={20} color={colors.mint} />
        <View className="flex-1">
          <Text variant="label">{t('travel.prefs.contacts')}</Text>
          <Text variant="caption">{t('travel.prefs.contactsBody')}</Text>
        </View>
        <ChevronRight size={18} color={colors.faint} />
      </Pressable>
    </Screen>
  );
}
