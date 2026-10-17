import { useQuery } from '@tanstack/react-query';
import { View } from 'react-native';

import { get } from '@/api/client';
import type { Vibe } from '@/api/types';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';

const CATEGORIES = ['talk_style', 'music', 'sport', 'hobby', 'work', 'study', 'food', 'wellbeing'] as const;

export const MAX_VIBES = 10;
export const MIN_VIBES = 3;

export function useVibeCatalog() {
  return useQuery({
    queryKey: ['vibe-catalog'],
    queryFn: () => get<{ items: Vibe[] }>('/v1/vibes').then((r) => r.items),
    staleTime: Infinity,
  });
}

export function VibePicker({ selected, onChange, locale: localeProp }: { selected: string[]; onChange: (s: string[]) => void; locale?: 'en' | 'hi' }) {
  const { t, locale: appLocale } = useT();
  const locale = localeProp ?? appLocale;
  const { data = [] } = useVibeCatalog();
  const groups = data.reduce<Record<string, Vibe[]>>((acc, v) => {
    (acc[v.category] ??= []).push(v);
    return acc;
  }, {});

  function toggle(slug: string) {
    if (selected.includes(slug)) onChange(selected.filter((s) => s !== slug));
    else if (selected.length < MAX_VIBES) onChange([...selected, slug]);
  }

  return (
    <View className="gap-5">
      {CATEGORIES
        .filter((cat) => groups[cat])
        .map((cat) => [cat, groups[cat]] as const)
        .map(([cat, vibes]) => (
        <View key={cat} className="gap-2">
          <Text variant="eyebrow">
            {t(`onboarding.vibeCategories.${cat}`)}
          </Text>
          <View className="flex-row flex-wrap gap-2">
            {vibes.map((v) => (
              <Chip key={v.slug} label={locale === 'hi' ? v.label_hi : v.label_en} selected={selected.includes(v.slug)} onPress={() => toggle(v.slug)} />
            ))}
          </View>
        </View>
      ))}
    </View>
  );
}
