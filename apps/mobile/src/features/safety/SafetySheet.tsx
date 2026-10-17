import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { BellOff, Flag, LifeBuoy, ShieldBan } from 'lucide-react-native';
import { createContext, useCallback, useContext, useState, type ReactNode } from 'react';
import { Alert, Modal, Pressable, ScrollView, Switch, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { post } from '@/api/client';
import type { ContentType, ReportReason } from '@/api/types';
import { Helplines } from '@/components/Helplines';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Text } from '@/components/ui/Text';
import { toast } from '@/components/ui/Toast';
import { colors } from '@/lib/theme';

export type SafetyTarget = {
  type: ContentType;
  id: string;
  personaId?: string;
  alias?: string;
};

const REASONS: { key: ReportReason; label: string; hint: string }[] = [
  { key: 'harassment', label: 'Harassment or bullying', hint: 'Targeting, insulting or stalking someone' },
  { key: 'sexual', label: 'Sexual content', hint: 'Unwanted sexual messages or intimate images' },
  { key: 'threat', label: 'Threat or violence', hint: 'Threatening to hurt someone' },
  { key: 'doxxing', label: 'Sharing private info', hint: 'Phone numbers, addresses, real names' },
  { key: 'hate', label: 'Hate speech', hint: 'Attacks on religion, caste, gender, region…' },
  { key: 'self_harm', label: 'Someone may be at risk', hint: 'Talk of self-harm or suicide' },
  { key: 'underage', label: 'Under 18', hint: 'This person seems to be a minor' },
  { key: 'impersonation', label: 'Impersonation', hint: 'Pretending to be someone else' },
  { key: 'spam', label: 'Spam or scam', hint: 'Ads, links, money requests' },
  { key: 'other', label: 'Something else', hint: '' },
];

type Ctx = { open: (t: SafetyTarget) => void };
const SafetyCtx = createContext<Ctx>({ open: () => {} });

export function useSafetySheet() {
  return useContext(SafetyCtx);
}

type Step = 'menu' | 'reasons' | 'done';

export function SafetySheetProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const [target, setTarget] = useState<SafetyTarget | null>(null);
  const [step, setStep] = useState<Step>('menu');
  const [reason, setReason] = useState<ReportReason | null>(null);
  const [details, setDetails] = useState('');
  const [alsoBlock, setAlsoBlock] = useState(false);
  const [support, setSupport] = useState(false);

  const open = useCallback((t: SafetyTarget) => {
    setTarget(t);
    setStep('menu');
    setReason(null);
    setDetails('');
    setAlsoBlock(false);
    setSupport(false);
  }, []);
  const close = () => setTarget(null);

  const report = useMutation({
    mutationFn: () =>
      post<{ show_support: boolean }>('/v1/reports', {
        target_type: target!.type,
        target_id: target!.id,
        reason,
        details,
        also_block: alsoBlock,
      }),
    onSuccess: (r) => {
      setSupport(r.show_support);
      setStep('done');
      if (alsoBlock) qc.invalidateQueries();
    },
  });

  const block = useMutation({
    mutationFn: () => post('/v1/blocks', { persona_id: target!.personaId }),
    onSuccess: () => {
      toast(`${target?.alias ?? 'They'} can no longer see you or contact you.`, 'success');
      qc.invalidateQueries();
      close();
    },
  });

  const mute = useMutation({
    mutationFn: () => post('/v1/mutes', { persona_id: target!.personaId }),
    onSuccess: () => {
      toast(`You won't see ${target?.alias ?? 'them'} anymore.`, 'success');
      qc.invalidateQueries();
      close();
    },
  });

  function confirmBlock() {
    Alert.alert(`Block ${target?.alias ?? 'this person'}?`, 'You won’t see each other anywhere on Metro Buddy. They won’t be told.', [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Block', style: 'destructive', onPress: () => block.mutate() },
    ]);
  }

  const row = (icon: ReactNode, label: string, onPress: () => void, danger?: boolean) => (
    <Pressable onPress={onPress} className="flex-row items-center gap-4 rounded-xl px-2 py-3.5 active:bg-raised" accessibilityRole="button">
      {icon}
      <Text className={danger ? 'text-danger' : 'text-ink'}>{label}</Text>
    </Pressable>
  );

  return (
    <SafetyCtx.Provider value={{ open }}>
      {children}
      <Modal visible={!!target} transparent animationType="slide" onRequestClose={close}>
        <Pressable className="flex-1 bg-black/60" onPress={close} accessibilityLabel="Close" />
        <View className="max-h-[85%] rounded-t-2xl border-t border-line bg-surface px-5 pt-3" style={{ paddingBottom: insets.bottom + 16 }}>
          <View className="mb-3 h-1 w-10 self-center rounded-full bg-line" />
          {step === 'menu' && target ? (
            <View>
              {row(<Flag color={colors.danger} size={20} />, 'Report', () => setStep('reasons'), true)}
              {target.personaId ? (
                <>
                  {row(<ShieldBan color={colors.ink} size={20} />, `Block ${target.alias ?? ''}`.trim(), confirmBlock)}
                  {row(<BellOff color={colors.ink} size={20} />, `Mute ${target.alias ?? ''}`.trim(), () => mute.mutate())}
                </>
              ) : null}
              {row(<LifeBuoy color={colors.mint} size={20} />, 'Safety Center & helplines', () => {
                close();
                router.push('/safety');
              })}
            </View>
          ) : null}

          {step === 'reasons' ? (
            <ScrollView keyboardShouldPersistTaps="handled">
              <Text variant="heading" className="mb-1">
                What’s wrong?
              </Text>
              <Text variant="caption" className="mb-3">
                Reports are anonymous. A person on our safety team reviews every one.
              </Text>
              <View className="gap-1.5">
                {REASONS.map((r) => (
                  <Pressable
                    key={r.key}
                    onPress={() => setReason(r.key)}
                    className={`rounded-xl border px-4 py-3 ${reason === r.key ? 'border-accent bg-accent/10' : 'border-line'}`}
                    accessibilityState={{ selected: reason === r.key }}
                  >
                    <Text variant="label">{r.label}</Text>
                    {r.hint ? <Text variant="caption">{r.hint}</Text> : null}
                  </Pressable>
                ))}
              </View>
              <View className="mt-4">
                <Input placeholder="Anything else we should know? (optional)" value={details} onChangeText={(t) => setDetails(t.slice(0, 500))} multiline />
              </View>
              {target?.personaId ? (
                <View className="mt-4 flex-row items-center justify-between">
                  <Text>Also block {target.alias}</Text>
                  <Switch value={alsoBlock} onValueChange={setAlsoBlock} trackColor={{ true: colors.accent, false: colors.line }} thumbColor={colors.ink} />
                </View>
              ) : null}
              <Button title="Send report" className="mb-2 mt-5" disabled={!reason} loading={report.isPending} onPress={() => report.mutate()} />
            </ScrollView>
          ) : null}

          {step === 'done' ? (
            <ScrollView>
              <Text variant="heading" className="mb-1">
                Thanks for telling us
              </Text>
              <Text className="mb-4 text-muted">
                {support
                  ? 'If someone is in immediate danger, call 112. You can also share these free, confidential helplines.'
                  : 'Our team will review this. We’ll keep the person who reported anonymous.'}
              </Text>
              {support ? <Helplines only={['112', '14416']} /> : null}
              <Button title="Done" kind="secondary" className="mb-2 mt-5" onPress={close} />
            </ScrollView>
          ) : null}
        </View>
      </Modal>
    </SafetyCtx.Provider>
  );
}
