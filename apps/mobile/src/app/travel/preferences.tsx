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
import { colors } from '@/lib/theme';

type Settings = { match_same_gender_only: boolean; match_verified_only: boolean; match_age_min: number | null; match_age_max: number | null };

const BANDS: { label: string; min: number | null; max: number | null }[] = [
  { label: 'Any age', min: null, max: null },
  { label: '18–25', min: 18, max: 25 },
  { label: '22–30', min: 22, max: 30 },
  { label: '28–40', min: 28, max: 40 },
  { label: '35+', min: 35, max: 80 },
];

const GENDERS = [
  { key: 'woman', label: 'Woman' },
  { key: 'man', label: 'Man' },
  { key: 'non_binary', label: 'Non-binary' },
  { key: 'prefer_not_to_say', label: 'Prefer not to say' },
];

export default function Preferences() {
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
      <ScreenHeader title="Matching preferences" />
      <Text className="text-muted">These only filter who you’re matched with. They’re never shown to anyone.</Text>
      <View className="gap-2">
        <Text variant="label">Your gender (private)</Text>
        <View className="flex-row flex-wrap gap-2">
          {GENDERS.map((g) => (
            <Chip key={g.key} label={g.label} selected={me?.gender === g.key} onPress={() => setGender.mutate(g.key)} />
          ))}
        </View>
      </View>
      {[
        { key: 'match_same_gender_only', label: 'Same gender only', body: 'Only match with people of your gender. Needs your gender set above.' },
        { key: 'match_verified_only', label: 'Verified only', body: 'Only match with people who completed selfie verification.' },
      ].map((row) => (
        <View key={row.key} className="flex-row items-center gap-4 rounded-2xl border border-line bg-surface p-4">
          <View className="flex-1 gap-1">
            <Text variant="label">{row.label}</Text>
            <Text variant="caption">{row.body}</Text>
          </View>
          <Switch
            value={!!s[row.key as keyof Settings]}
            onValueChange={(v) => save.mutate({ [row.key]: v })}
            trackColor={{ true: colors.accent, false: colors.line }}
            thumbColor={colors.ink}
          />
        </View>
      ))}
      <View className="gap-2">
        <Text variant="label">Age range</Text>
        <View className="flex-row flex-wrap gap-2">
          {BANDS.map((b) => (
            <Chip key={b.label} label={b.label} selected={b === band} onPress={() => save.mutate(b.min ? { match_age_min: b.min, match_age_max: b.max } : { clear_age_band: true })} />
          ))}
        </View>
      </View>
      <Pressable onPress={() => router.push('/settings/contacts')} className="flex-row items-center gap-3 rounded-2xl border border-line bg-surface p-4">
        <Users size={20} color={colors.mint} />
        <View className="flex-1">
          <Text variant="label">Trusted contacts</Text>
          <Text variant="caption">Who gets your SOS text and live trip link</Text>
        </View>
        <ChevronRight size={18} color={colors.faint} />
      </Pressable>
    </Screen>
  );
}
