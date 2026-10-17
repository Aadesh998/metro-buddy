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
import { useT } from '@/i18n';
import { colors } from '@/lib/theme';

export type SafetyTarget = {
  type: ContentType;
  id: string;
  personaId?: string;
  alias?: string;
};

const REASONS: ReportReason[] = ['harassment', 'sexual', 'threat', 'doxxing', 'hate', 'self_harm', 'underage', 'impersonation', 'spam', 'other'];

type Ctx = { open: (t: SafetyTarget) => void };
const SafetyCtx = createContext<Ctx>({ open: () => {} });

export function useSafetySheet() {
  return useContext(SafetyCtx);
}

type Step = 'menu' | 'reasons' | 'done';

export function SafetySheetProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const insets = useSafeAreaInsets();
  const { t } = useT();
  const [target, setTarget] = useState<SafetyTarget | null>(null);
  const [step, setStep] = useState<Step>('menu');
  const [reason, setReason] = useState<ReportReason | null>(null);
  const [details, setDetails] = useState('');
  const [alsoBlock, setAlsoBlock] = useState(false);
  const [support, setSupport] = useState(false);

  const open = useCallback((next: SafetyTarget) => {
    setTarget(next);
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
      toast(t('safety.sheet.blockedToast', { name: target?.alias ?? t('safety.sheet.they') }), 'success');
      qc.invalidateQueries();
      close();
    },
  });

  const mute = useMutation({
    mutationFn: () => post('/v1/mutes', { persona_id: target!.personaId }),
    onSuccess: () => {
      toast(t('safety.sheet.mutedToast', { name: target?.alias ?? t('safety.sheet.them') }), 'success');
      qc.invalidateQueries();
      close();
    },
  });

  function confirmBlock() {
    Alert.alert(t('safety.sheet.blockTitle', { name: target?.alias ?? t('safety.sheet.thisPerson') }), t('safety.sheet.blockBody'), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('common.block'), style: 'destructive', onPress: () => block.mutate() },
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
        <Pressable className="flex-1 bg-black/60" onPress={close} accessibilityLabel={t('common.close')} />
        <View className="max-h-[85%] rounded-t-2xl border-t border-line bg-surface px-5 pt-3" style={{ paddingBottom: insets.bottom + 16 }}>
          <View className="mb-3 h-1 w-10 self-center rounded-full bg-line" />
          {step === 'menu' && target ? (
            <View>
              {row(<Flag color={colors.danger} size={20} />, t('common.report'), () => setStep('reasons'), true)}
              {target.personaId ? (
                <>
                  {row(<ShieldBan color={colors.ink} size={20} />, target.alias ? t('safety.sheet.blockName', { name: target.alias }) : t('common.block'), confirmBlock)}
                  {row(<BellOff color={colors.ink} size={20} />, target.alias ? t('safety.sheet.muteName', { name: target.alias }) : t('common.mute'), () => mute.mutate())}
                </>
              ) : null}
              {row(<LifeBuoy color={colors.mint} size={20} />, t('safety.sheet.centerLink'), () => {
                close();
                router.push('/safety');
              })}
            </View>
          ) : null}

          {step === 'reasons' ? (
            <ScrollView keyboardShouldPersistTaps="handled">
              <Text variant="heading" className="mb-1">
                {t('safety.sheet.whatsWrong')}
              </Text>
              <Text variant="caption" className="mb-3">
                {t('safety.sheet.anonymous')}
              </Text>
              <View className="gap-1.5">
                {REASONS.map((r) => {
                  const hint = t(`safety.reasons.${r}.hint`);
                  return (
                    <Pressable
                      key={r}
                      onPress={() => setReason(r)}
                      className={`rounded-xl border px-4 py-3 ${reason === r ? 'border-accent bg-accent/10' : 'border-line'}`}
                      accessibilityState={{ selected: reason === r }}
                    >
                      <Text variant="label">{t(`safety.reasons.${r}.label`)}</Text>
                      {hint ? <Text variant="caption">{hint}</Text> : null}
                    </Pressable>
                  );
                })}
              </View>
              <View className="mt-4">
                <Input placeholder={t('safety.sheet.detailsPlaceholder')} value={details} onChangeText={(s) => setDetails(s.slice(0, 500))} multiline />
              </View>
              {target?.personaId ? (
                <View className="mt-4 flex-row items-center justify-between">
                  <Text>{t('safety.sheet.alsoBlock', { name: target.alias ?? '' })}</Text>
                  <Switch value={alsoBlock} onValueChange={setAlsoBlock} trackColor={{ true: colors.accent, false: colors.line }} thumbColor={colors.ink} />
                </View>
              ) : null}
              <Button title={t('safety.sheet.sendReport')} className="mb-2 mt-5" disabled={!reason} loading={report.isPending} onPress={() => report.mutate()} />
            </ScrollView>
          ) : null}

          {step === 'done' ? (
            <ScrollView>
              <Text variant="heading" className="mb-1">
                {t('safety.sheet.thanks')}
              </Text>
              <Text className="mb-4 text-muted">
                {support
                  ? t('safety.sheet.supportBody')
                  : t('safety.sheet.reviewBody')}
              </Text>
              {support ? <Helplines only={['112', '14416']} /> : null}
              <Button title={t('common.done')} kind="secondary" className="mb-2 mt-5" onPress={close} />
            </ScrollView>
          ) : null}
        </View>
      </Modal>
    </SafetyCtx.Provider>
  );
}
