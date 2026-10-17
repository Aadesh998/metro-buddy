import { Check } from 'lucide-react-native';
import { Pressable, View } from 'react-native';

import { Avatar } from '@/components/Avatar';
import { Text } from '@/components/ui/Text';
import { useT } from '@/i18n';
import { AVATAR_IDS } from '@/lib/avatars';
import { colors } from '@/lib/theme';

// Grid of the bundled illustrated avatars, split into two rows of ten.
export function AvatarPicker({ value, onChange, size = 52 }: { value?: string; onChange: (seed: string) => void; size?: number }) {
  const { t } = useT();
  const groups = [
    { label: t('onboarding.persona.avatarsMen'), ids: AVATAR_IDS.filter((id) => id.startsWith('img-m')) },
    { label: t('onboarding.persona.avatarsWomen'), ids: AVATAR_IDS.filter((id) => id.startsWith('img-f')) },
  ];
  return (
    <View className="gap-3">
      {groups.map((g) => (
        <View key={g.label} className="gap-2">
          <Text variant="caption">{g.label}</Text>
          <View className="flex-row flex-wrap gap-2">
            {g.ids.map((id) => {
              const on = value === id;
              return (
                <Pressable
                  key={id}
                  onPress={() => onChange(id)}
                  className={`rounded-full border-2 p-0.5 ${on ? 'border-brand' : 'border-transparent'}`}
                  accessibilityRole="button"
                  accessibilityState={{ selected: on }}
                  accessibilityLabel={t('onboarding.persona.avatarN', { n: id.slice(-2) })}
                >
                  <Avatar seed={id} size={size} />
                  {on ? (
                    <View className="absolute bottom-0 right-0 h-5 w-5 items-center justify-center rounded-full bg-brand">
                      <Check size={12} color={colors.surface} />
                    </View>
                  ) : null}
                </Pressable>
              );
            })}
          </View>
        </View>
      ))}
    </View>
  );
}
