import { FlashList } from "@shopify/flash-list";
import { useLocalSearchParams } from "expo-router";
import { Users } from "lucide-react-native";
import { useState } from "react";
import { Pressable, ScrollView, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import type { PersonaRef } from "@/api/types";
import { Avatar } from "@/components/Avatar";
import { EmptyState } from "@/components/EmptyState";
import { PersonaName } from "@/components/PersonaBadge";
import { ScreenHeader } from "@/components/ScreenHeader";
import { Chip } from "@/components/ui/Chip";
import { Text } from "@/components/ui/Text";
import { useAuthorSheet } from "@/features/social/AuthorSheet";
import {
  useConnections,
  useFollows,
  useRespond,
  type Connection,
} from "@/features/social/hooks";
import { useT } from "@/i18n";
import { colors } from "@/lib/theme";

const TABS = [
  "requests",
  "connections",
  "sent",
  "followers",
  "following",
] as const;
type Tab = (typeof TABS)[number];

// Actions sit under the name so long aliases and Hindi labels never squeeze each other.
function PersonRow({
  p,
  note,
  below,
}: {
  p: PersonaRef;
  note?: string;
  below?: React.ReactNode;
}) {
  const author = useAuthorSheet();
  return (
    <Pressable
      onPress={() => author.open({ persona: p })}
      className="flex-row gap-3 px-4 py-3 active:bg-raised"
      accessibilityRole="button"
    >
      <Avatar seed={p.avatar_seed} size={44} />
      <View className="min-w-0 flex-1 gap-1">
        <PersonaName p={p} />
        {note ? (
          <Text variant="caption" numberOfLines={3}>
            {note}
          </Text>
        ) : null}
        {below}
      </View>
    </Pressable>
  );
}

function RequestButtons({ c }: { c: Connection }) {
  const { t } = useT();
  const respond = useRespond();
  return (
    <View className="mt-1 flex-row gap-2">
      <Pressable
        onPress={() => respond.mutate({ connectionId: c.id, accept: false })}
        disabled={respond.isPending}
        className="h-9 justify-center rounded-full border border-line px-3 active:bg-raised"
      >
        <Text className="text-sm font-semibold">{t("social.decline")}</Text>
      </Pressable>
      <Pressable
        onPress={() => respond.mutate({ connectionId: c.id, accept: true })}
        disabled={respond.isPending}
        className="h-9 justify-center rounded-full bg-accent px-3 active:opacity-80"
      >
        <Text className="text-sm font-semibold text-accent-ink">
          {t("social.accept")}
        </Text>
      </Pressable>
    </View>
  );
}

function ConnList({
  box,
  tab,
}: {
  box: "incoming" | "accepted" | "outgoing";
  tab: Tab;
}) {
  const { t } = useT();
  const q = useConnections(box);
  return (
    <FlashList
      data={q.data ?? []}
      keyExtractor={(c) => c.id}
      renderItem={({ item: c }) => (
        <PersonRow
          p={c.persona}
          note={c.note || undefined}
          below={box === "incoming" ? <RequestButtons c={c} /> : undefined}
        />
      )}
      ListEmptyComponent={
        q.isPending ? null : (
          <EmptyState
            icon={<Users size={28} color={colors.faint} />}
            title={t(`social.empty.${tab}`)}
          />
        )
      }
    />
  );
}

function FollowList({ kind }: { kind: "followers" | "following" }) {
  const { t } = useT();
  const q = useFollows(kind);
  return (
    <FlashList
      data={q.data ?? []}
      keyExtractor={(p) => p.id}
      renderItem={({ item }) => <PersonRow p={item} />}
      ListEmptyComponent={
        q.isPending ? null : (
          <EmptyState
            icon={<Users size={28} color={colors.faint} />}
            title={t(`social.empty.${kind}`)}
          />
        )
      }
    />
  );
}

export default function ConnectionsScreen() {
  const { t } = useT();
  const params = useLocalSearchParams<{ tab?: Tab }>();
  const [tab, setTab] = useState<Tab>(
    params.tab && TABS.includes(params.tab) ? params.tab : "requests",
  );
  const incoming = useConnections("incoming").data?.length ?? 0;
  return (
    <SafeAreaView edges={["top"]} className="flex-1 bg-bg">
      <ScreenHeader title={t("social.screenTitle")} />
      <View>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerClassName="gap-2 px-4 pb-3"
        >
          {TABS.map((k) => (
            <Chip
              key={k}
              label={
                k === "requests" && incoming > 0
                  ? `${t(`social.tabs.${k}`)} · ${incoming}`
                  : t(`social.tabs.${k}`)
              }
              selected={tab === k}
              onPress={() => setTab(k)}
            />
          ))}
        </ScrollView>
      </View>
      <View className="flex-1">
        {tab === "requests" ? <ConnList box="incoming" tab={tab} /> : null}
        {tab === "connections" ? <ConnList box="accepted" tab={tab} /> : null}
        {tab === "sent" ? <ConnList box="outgoing" tab={tab} /> : null}
        {tab === "followers" ? <FollowList kind="followers" /> : null}
        {tab === "following" ? <FollowList kind="following" /> : null}
      </View>
    </SafeAreaView>
  );
}
