import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Image } from 'expo-image';
import * as ImagePicker from 'expo-image-picker';
import { useState } from 'react';
import { View } from 'react-native';

import { get, put } from '@/api/client';
import type { RevealCard } from '@/api/types';
import { ScreenHeader } from '@/components/ScreenHeader';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Screen } from '@/components/ui/Screen';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { uploadImage } from '@/features/bowls/upload';

function CardForm({ card }: { card: RevealCard }) {
  const qc = useQueryClient();
  const [name, setName] = useState(card.first_name);
  const [contact, setContact] = useState(card.contact);
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => put<RevealCard>('/v1/me/reveal-card', body),
    onSuccess: (c) => {
      qc.setQueryData(['reveal-card'], c);
      qc.invalidateQueries({ queryKey: ['reveal'] });
      toast('Saved privately', 'success');
    },
    onError: (e) => toast(e.message),
  });
  async function pickPhoto() {
    const r = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ['images'], quality: 0.8, allowsEditing: true, aspect: [1, 1] });
    if (r.canceled) return;
    const a = r.assets[0];
    const key = await uploadImage({ uri: a.uri, mimeType: a.mimeType, fileSize: a.fileSize });
    save.mutate({ photo_key: key });
  }
  return (
    <>
      <Input label="First name" value={name} onChangeText={(t) => setName(t.slice(0, 30))} placeholder="Only your first name" />
      <View className="gap-2">
        <Text variant="label">Photo</Text>
        <View className="flex-row items-center gap-3">
          {card.photo_url ? <Image source={{ uri: card.photo_url }} style={{ width: 64, height: 64, borderRadius: 32 }} /> : null}
          <Button title={card.photo_url ? 'Change photo' : 'Add photo'} kind="secondary" className="h-10 px-4" onPress={pickPhoto} />
          {card.photo_url ? <Button title="Remove" kind="ghost" className="h-10" onPress={() => save.mutate({ remove_photo: true })} /> : null}
        </View>
      </View>
      <Input label="Contact" value={contact} onChangeText={(t) => setContact(t.slice(0, 80))} placeholder="Phone number or a social handle" />
      <Button title="Save" loading={save.isPending} disabled={name === card.first_name && contact === card.contact} onPress={() => save.mutate({ first_name: name, contact })} />
    </>
  );
}

export default function RevealCardScreen() {
  const card = useQuery({ queryKey: ['reveal-card'], queryFn: () => get<RevealCard>('/v1/me/reveal-card') });
  return (
    <Screen scroll className="gap-5">
      <ScreenHeader title="Reveal card" />
      <Text className="text-muted">
        These stay encrypted and hidden. In a one-to-one chat, you and the other person can each choose to reveal one step at a time: first name, then
        photo, then contact. Nothing is shown until you both agree, and you can step back whenever you like.
      </Text>
      {card.data ? <CardForm key={JSON.stringify(card.data)} card={card.data} /> : null}
    </Screen>
  );
}
