import AsyncStorage from '@react-native-async-storage/async-storage';
import Constants from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import { AndroidImportance } from 'expo-notifications';
import { router } from 'expo-router';
import { Platform } from 'react-native';

import { del, post } from '@/api/client';
import { activeConversation } from '@/features/chat/store';
import { t } from '@/i18n';

const ASKED = 'mb.push_asked';
let token: string | null = null;

// Banners for the chat you're already looking at would just be noise.
Notifications.setNotificationHandler({
  handleNotification: async (n) => {
    const link = String(n.request.content.data?.link ?? '');
    const here = activeConversation.get();
    const quiet = !!here && link.endsWith(`/chat/${here}`);
    return { shouldShowBanner: !quiet, shouldShowList: !quiet, shouldPlaySound: !quiet, shouldSetBadge: false };
  },
});

export function linkToPath(link: string): string | null {
  const m = link.match(/^metrobuddy:\/\/(.+)$/);
  return m ? `/${m[1]}` : null;
}

export function listenForTaps() {
  const sub = Notifications.addNotificationResponseReceivedListener((r) => {
    const path = linkToPath(String(r.notification.request.content.data?.link ?? ''));
    if (path) router.push(path as never);
  });
  return () => sub.remove();
}

async function ensureChannels() {
  if (Platform.OS !== 'android') return;
  const channels = [
    ['message', AndroidImportance.HIGH],
    ['dm_request', AndroidImportance.DEFAULT],
    ['reply', AndroidImportance.DEFAULT],
    ['match', AndroidImportance.HIGH],
    ['trip_reminder', AndroidImportance.HIGH],
    ['episode_live', AndroidImportance.DEFAULT],
    ['mod_notice', AndroidImportance.DEFAULT],
    ['sos', AndroidImportance.MAX],
  ] as const;
  await Promise.all(
    channels.map(([id, importance]) => Notifications.setNotificationChannelAsync(id, { name: t(`safety.push.channels.${id}`), importance })),
  );
}

export async function pushStatus(): Promise<'granted' | 'denied' | 'undetermined' | 'unsupported'> {
  if (!Device.isDevice || Platform.OS === 'web') return 'unsupported';
  const p = await Notifications.getPermissionsAsync();
  return p.granted ? 'granted' : p.canAskAgain ? 'undetermined' : 'denied';
}

export async function wasAsked() {
  return (await AsyncStorage.getItem(ASKED)) === '1';
}

export async function registerForPush(ask: boolean) {
  if ((await pushStatus()) === 'unsupported') return false;
  await ensureChannels();
  let perm = await Notifications.getPermissionsAsync();
  if (!perm.granted && ask) {
    await AsyncStorage.setItem(ASKED, '1');
    perm = await Notifications.requestPermissionsAsync();
  }
  if (!perm.granted) return false;
  const projectId = Constants.expoConfig?.extra?.eas?.projectId ?? Constants.easConfig?.projectId;
  if (!projectId) return false;
  token = (await Notifications.getExpoPushTokenAsync({ projectId })).data;
  await post('/v1/me/push-token', {
    token,
    platform: Platform.OS === 'ios' ? 'ios' : 'android',
    app_version: Constants.expoConfig?.version ?? '',
  });
  return true;
}

export async function unregisterPush() {
  if (!token) return;
  await del(`/v1/me/push-token`).catch(() => {});
  token = null;
}
