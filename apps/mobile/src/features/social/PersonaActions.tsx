import {
  Check,
  MessageCircle,
  UserCheck,
  UserPlus,
  Users,
} from "lucide-react-native";
import { useState, type ReactNode } from "react";
import { ActivityIndicator, Alert, Modal, Pressable, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError } from "@/api/client";
import type { PersonaRef } from "@/api/types";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Text } from "@/components/ui/Text";
import { toast } from "@/components/ui/Toast";
import { useT } from "@/i18n";
import { colors } from "@/lib/theme";
import {
  useConnect,
  useFollow,
  useMessage,
  useRelationship,
  useRemoveConnection,
  useRespond,
} from "./hooks";

type Props = {
  persona: Pick<PersonaRef, "id" | "alias" | "avatar_seed">;
  // Where the chat invite came from, so the request shows it ("about your comment").
  context?: "post" | "comment" | "room" | "profile" | "episode";
  contextId?: string;
  onNavigate?: () => void;
};

function Action({
  icon,
  label,
  onPress,
  active,
  busy,
}: {
  icon: ReactNode;
  label: string;
  onPress: () => void;
  active?: boolean;
  busy?: boolean;
}) {
  return (
    <Pressable
      onPress={onPress}
      disabled={busy}
      accessibilityRole="button"
      className={`h-11 min-w-0 flex-1 flex-row items-center justify-center gap-1.5 rounded-full px-3 ${active ? "border border-line bg-raised" : "bg-accent"} active:opacity-80`}
    >
      {busy ? (
        <ActivityIndicator
          size="small"
          color={active ? colors.ink : colors.accentInk}
        />
      ) : (
        icon
      )}
      <Text
        numberOfLines={1}
        className={`shrink text-sm font-semibold ${active ? "text-ink" : "text-accent-ink"}`}
      >
        {label}
      </Text>
    </Pressable>
  );
}

const fail = (e: unknown, fallback: string) =>
  toast(e instanceof ApiError ? e.message : fallback, "error");

// Follow, connect and message buttons for another persona.
export function PersonaActions({
  persona,
  context = "profile",
  contextId,
  onNavigate,
}: Props) {
  const { t } = useT();
  const insets = useSafeAreaInsets();
  const rel = useRelationship(persona.id);
  const follow = useFollow();
  const connect = useConnect();
  const respond = useRespond();
  const remove = useRemoveConnection();
  const message = useMessage();
  const [asking, setAsking] = useState(false);
  const [note, setNote] = useState("");
  const r = rel.data;
  const failed = t("chat.rooms.failed");

  const onConnect = () => {
    if (!r) return;
    if (r.connection === "none") {
      setNote("");
      setAsking(true);
    } else if (r.connection === "incoming" && r.connection_id) {
      respond.mutate(
        { connectionId: r.connection_id, accept: true },
        {
          onSuccess: () =>
            toast(
              t("social.nowConnected", { alias: persona.alias }),
              "success",
            ),
          onError: (e) => fail(e, failed),
        },
      );
    } else if (r.connection_id) {
      const id = r.connection_id;
      const connected = r.connection === "connected";
      Alert.alert(
        connected
          ? t("social.removeTitle", { alias: persona.alias })
          : t("social.withdraw"),
        connected ? t("social.removeBody") : undefined,
        [
          { text: t("common.cancel"), style: "cancel" },
          {
            text: connected ? t("social.remove") : t("social.withdraw"),
            style: "destructive",
            onPress: () => remove.mutate(id),
          },
        ],
      );
    }
  };

  const sendRequest = () =>
    connect.mutate(
      { id: persona.id, note: note.trim() },
      {
        onSuccess: (res) => {
          setAsking(false);
          const done =
            (res as { connection?: string }).connection === "connected";
          toast(
            done
              ? t("social.nowConnected", { alias: persona.alias })
              : t("social.requestSent", { alias: persona.alias }),
            "success",
          );
        },
        onError: (e) => fail(e, failed),
      },
    );

  const connLabel = !r
    ? t("social.connect")
    : {
        none: t("social.connect"),
        outgoing: t("social.requested"),
        incoming: t("social.accept"),
        connected: t("social.connected"),
      }[r.connection];
  const connActive =
    r?.connection === "outgoing" || r?.connection === "connected";

  return (
    <View className="w-full gap-2">
      <View className="w-full flex-row gap-2">
        <Action
          icon={
            r?.following ? (
              <UserCheck size={16} color={colors.ink} />
            ) : (
              <UserPlus size={16} color={colors.accentInk} />
            )
          }
          label={r?.following ? t("social.following") : t("social.follow")}
          active={r?.following}
          busy={follow.isPending}
          onPress={() =>
            r &&
            follow.mutate(
              { id: persona.id, on: !r.following },
              { onError: (e) => fail(e, failed) },
            )
          }
        />
        <Action
          icon={
            connActive ? (
              <Check size={16} color={colors.ink} />
            ) : (
              <Users size={16} color={colors.accentInk} />
            )
          }
          label={connLabel}
          active={connActive}
          busy={connect.isPending || respond.isPending || remove.isPending}
          onPress={onConnect}
        />
      </View>
      <View className="w-full flex-row">
        <Action
          icon={<MessageCircle size={16} color={colors.ink} />}
          label={t("social.message")}
          active
          busy={message.isPending}
          onPress={() => {
            onNavigate?.();
            message.mutate(
              { ...persona, context, context_id: contextId },
              {
                onError: (e) =>
                  !(e instanceof ApiError && e.code === "not_connected") &&
                  fail(e, failed),
              },
            );
          }}
        />
      </View>
      {r?.follows_you ? (
        <Text variant="caption" className="text-center">
          {t("social.followsYou")}
        </Text>
      ) : null}

      <Modal
        visible={asking}
        transparent
        animationType="slide"
        onRequestClose={() => setAsking(false)}
      >
        <Pressable
          className="flex-1 bg-black/60"
          onPress={() => setAsking(false)}
          accessibilityLabel={t("common.close")}
        />
        <View
          className="gap-3 rounded-t-2xl border-t border-line bg-surface px-5 pt-3"
          style={{ paddingBottom: insets.bottom + 16 }}
        >
          <View className="mb-1 h-1 w-10 self-center rounded-full bg-line" />
          <Text variant="heading">
            {t("social.connectTitle", { alias: persona.alias })}
          </Text>
          <Text className="text-muted">{t("social.connectBody")}</Text>
          <Input
            value={note}
            onChangeText={(v) => setNote(v.slice(0, 140))}
            placeholder={t("social.notePlaceholder")}
            multiline
          />
          <Button
            title={t("social.sendRequest")}
            loading={connect.isPending}
            onPress={sendRequest}
          />
        </View>
      </Modal>
    </View>
  );
}
