import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Image } from 'expo-image';
import * as ImagePicker from 'expo-image-picker';
import { router, useLocalSearchParams } from 'expo-router';
import { BarChart3, EyeOff, ImagePlus, Plus, Type, X } from 'lucide-react-native';
import { useEffect, useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { FontTextInput } from '@/components/ui/FontTextInput';
import { post as apiPost } from '@/api/client';
import type { Post } from '@/api/types';
import { SupportSheet } from '@/components/SupportSheet';
import { Button } from '@/components/ui/Button';
import { Chip } from '@/components/ui/Chip';
import { Text } from '@/components/ui/Text';
import { useJoinedBowls } from '@/features/bowls/hooks';
import { uploadImage } from '@/features/bowls/upload';
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

const FLAIRS = ['Rant', 'Advice', 'Question', 'Wholesome', 'Funny', 'Lost & Found', 'PSA'];
type Kind = 'text' | 'image' | 'poll';
type Picked = { uri: string; mimeType?: string; fileSize?: number };

function useMaskedPreview(text: string) {
  const [debounced, setDebounced] = useState(text);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(text), 400);
    return () => clearTimeout(t);
  }, [text]);
  return useQuery({
    queryKey: ['pii-preview', debounced],
    queryFn: () => apiPost<{ masked: string; hidden: string[] }>('/v1/pii/preview', { text: debounced }),
    enabled: debounced.trim().length > 0,
    staleTime: Infinity,
  });
}

export default function Compose() {
  const params = useLocalSearchParams<{ bowl?: string }>();
  const { t } = useT();
  const qc = useQueryClient();
  const joined = useJoinedBowls();
  const [bowl, setBowl] = useState<string | undefined>(params.bowl);
  const [kind, setKind] = useState<Kind>('text');
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [flair, setFlair] = useState<string | null>(null);
  const [images, setImages] = useState<Picked[]>([]);
  const [options, setOptions] = useState(['', '']);
  const [hours, setHours] = useState(24);
  const [supportFor, setSupportFor] = useState<Post | null>(null);
  const preview = useMaskedPreview(`${title}\n${body}\n${options.join('\n')}`);
  const bowlId = bowl ?? joined.data?.[0]?.id;

  async function pick() {
    const r = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ['images'], allowsMultipleSelection: true, selectionLimit: 4 - images.length, quality: 0.85 });
    if (!r.canceled) setImages((xs) => [...xs, ...r.assets.map((a) => ({ uri: a.uri, mimeType: a.mimeType, fileSize: a.fileSize }))].slice(0, 4));
  }

  const submit = useMutation({
    mutationFn: async () => {
      const media_keys = kind === 'image' ? await Promise.all(images.map(uploadImage)) : undefined;
      return apiPost<Post>('/v1/posts', {
        bowl_id: bowlId,
        type: kind,
        title: title.trim(),
        body: body.trim(),
        flair,
        media_keys,
        poll: kind === 'poll' ? { options: options.map((o) => o.trim()).filter(Boolean), closes_in_hours: hours } : undefined,
      });
    },
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['feed'] });
      if (p.show_support) setSupportFor(p);
      else router.replace(`/post/${p.id}`);
    },
  });

  const validPoll = options.filter((o) => o.trim()).length >= 2;
  const ready = !!bowlId && title.trim().length > 0 && (kind !== 'image' || images.length > 0) && (kind !== 'poll' || validPoll);
  const hidden = preview.data?.hidden ?? [];

  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-bg">
      <View className="h-12 flex-row items-center justify-between px-4">
        <Pressable onPress={() => router.back()} hitSlop={10} accessibilityLabel={t('common.close')}>
          <X size={24} color={colors.ink} />
        </Pressable>
        <Text variant="heading">{t('bowls.compose.title')}</Text>
        <Button title={t('bowls.compose.submit')} className="h-9 px-4" disabled={!ready} loading={submit.isPending} onPress={() => submit.mutate()} />
      </View>
      <ScrollView contentContainerClassName="px-4 pb-10 gap-4" keyboardShouldPersistTaps="handled">
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2">
          {(joined.data ?? []).map((b) => (
            <Chip key={b.id} label={b.name} selected={b.id === bowlId} onPress={() => setBowl(b.id)} />
          ))}
        </ScrollView>
        {joined.data?.length === 0 ? <Text className="text-muted">{t('bowls.compose.joinFirst')}</Text> : null}

        <View className="flex-row gap-2">
          {(
            [
              ['text', 'bowls.compose.text', Type],
              ['image', 'bowls.compose.image', ImagePlus],
              ['poll', 'bowls.compose.poll', BarChart3],
            ] as const
          ).map(([k, label, Icon]) => (
            <Pressable key={k} onPress={() => setKind(k)} className={`flex-1 flex-row items-center justify-center gap-2 rounded-xl border py-2.5 ${kind === k ? 'border-accent bg-accent/10' : 'border-line'}`}>
              <Icon size={16} color={kind === k ? colors.accent : colors.muted} />
              <Text className={kind === k ? 'text-accent' : 'text-muted'}>{t(label)}</Text>
            </Pressable>
          ))}
        </View>

        <FontTextInput value={title} onChangeText={(v) => setTitle(v.slice(0, 140))} placeholder={t('bowls.compose.titlePlaceholder')} placeholderTextColor={colors.faint} className="text-xl font-semibold text-ink" multiline />
        <FontTextInput
          value={body}
          onChangeText={(v) => setBody(v.slice(0, 5000))}
          placeholder={kind === 'poll' ? t('bowls.compose.pollBodyPlaceholder') : t('bowls.compose.bodyPlaceholder')}
          placeholderTextColor={colors.faint}
          className="min-h-24 text-base leading-6 text-ink"
          multiline
          textAlignVertical="top"
        />

        {kind === 'image' ? (
          <View className="flex-row flex-wrap gap-2">
            {images.map((im, i) => (
              <View key={im.uri}>
                <Image source={{ uri: im.uri }} style={{ width: 96, height: 96, borderRadius: 12 }} />
                <Pressable onPress={() => setImages(images.filter((_, j) => j !== i))} className="absolute right-1 top-1 rounded-full bg-black/70 p-1" accessibilityLabel={t('bowls.compose.removeImage')}>
                  <X size={12} color="#fff" />
                </Pressable>
              </View>
            ))}
            {images.length < 4 ? (
              <Pressable onPress={pick} className="h-24 w-24 items-center justify-center rounded-xl border border-dashed border-line" accessibilityLabel={t('bowls.compose.addImage')}>
                <Plus size={22} color={colors.muted} />
              </Pressable>
            ) : null}
            <Text variant="caption" className="w-full">
              {t('bowls.compose.imageNote')}
            </Text>
          </View>
        ) : null}

        {kind === 'poll' ? (
          <View className="gap-2">
            {options.map((o, i) => (
              <FontTextInput
                key={i}
                value={o}
                onChangeText={(v) => setOptions(options.map((x, j) => (j === i ? v.slice(0, 60) : x)))}
                placeholder={t('bowls.compose.option', { n: i + 1 })}
                placeholderTextColor={colors.faint}
                className="rounded-xl border border-line bg-surface px-4 py-3 text-base text-ink"
              />
            ))}
            {options.length < 6 ? <Button title={t('bowls.compose.addOption')} kind="ghost" onPress={() => setOptions([...options, ''])} /> : null}
            <View className="flex-row gap-2">
              {[6, 24, 72, 168].map((h) => (
                <Chip key={h} label={h < 24 ? t('common.time.hours', { n: h }) : t('common.time.days', { n: h / 24 })} selected={hours === h} onPress={() => setHours(h)} />
              ))}
            </View>
          </View>
        ) : null}

        <View className="gap-2">
          <Text variant="caption" className="uppercase tracking-wider">
            {t('bowls.compose.flair')}
          </Text>
          <View className="flex-row flex-wrap gap-2">
            {FLAIRS.map((f) => (
              <Chip key={f} label={f} selected={flair === f} onPress={() => setFlair(flair === f ? null : f)} />
            ))}
          </View>
        </View>

        {hidden.length > 0 ? (
          <View className="gap-2 rounded-2xl border border-accent/40 bg-accent/5 p-4">
            <View className="flex-row items-center gap-2">
              <EyeOff size={16} color={colors.accent} />
              <Text variant="label" className="text-accent">
                {t('bowls.compose.hiddenTitle')}
              </Text>
            </View>
            <Text variant="caption">{t('bowls.compose.hiddenBody')}</Text>
            <Text className="text-sm leading-5">{preview.data?.masked.trim()}</Text>
          </View>
        ) : null}
      </ScrollView>
      <SupportSheet
        visible={!!supportFor}
        onClose={() => {
          const id = supportFor?.id;
          setSupportFor(null);
          if (id) router.replace(`/post/${id}`);
        }}
      />
    </SafeAreaView>
  );
}
