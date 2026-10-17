import { router } from "expo-router";
import { ChevronRight } from "lucide-react-native";
import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from "react";
import { Modal, Pressable, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import type { PersonaRef } from "@/api/types";
import { Avatar } from "@/components/Avatar";
import { PersonaName } from "@/components/PersonaBadge";
import { Text } from "@/components/ui/Text";
import { useT } from "@/i18n";
import { colors } from "@/lib/theme";
import { useRelationship } from "./hooks";
import { PersonaActions } from "./PersonaActions";

type Target = {
  persona: PersonaRef;
  context?: "post" | "comment" | "room" | "profile" | "episode";
  contextId?: string;
};
type Ctx = { open: (t: Target) => void };
const AuthorCtx = createContext<Ctx>({ open: () => {} });

// Tapping an author anywhere opens this: follow, connect or message in one more tap.
export function useAuthorSheet() {
  return useContext(AuthorCtx);
}

function Counts({ id }: { id: string }) {
  const { tp } = useT();
  const r = useRelationship(id).data;
  if (!r) return null;
  return (
    <Text variant="caption">
      {tp("social.followers", r.followers)} ·{" "}
      {tp("social.connections", r.connections)}
    </Text>
  );
}

export function AuthorSheetProvider({ children }: { children: ReactNode }) {
  const insets = useSafeAreaInsets();
  const { t } = useT();
  const [target, setTarget] = useState<Target | null>(null);
  const open = useCallback((next: Target) => setTarget(next), []);
  const close = () => setTarget(null);
  const p = target?.persona;

  return (
    <AuthorCtx.Provider value={{ open }}>
      {children}
      <Modal
        visible={!!target}
        transparent
        animationType="slide"
        onRequestClose={close}
      >
        <Pressable
          className="flex-1 bg-black/60"
          onPress={close}
          accessibilityLabel={t("common.close")}
        />
        <View
          className="gap-4 rounded-t-2xl border-t border-line bg-surface px-5 pt-3"
          style={{ paddingBottom: insets.bottom + 16 }}
        >
          <View className="h-1 w-10 self-center rounded-full bg-line" />
          {p ? (
            <>
              <Pressable
                onPress={() => {
                  close();
                  router.push(`/u/${p.id}`);
                }}
                className="flex-row items-center gap-3 rounded-xl active:bg-raised"
                accessibilityRole="button"
                accessibilityLabel={t("social.viewProfile")}
              >
                <Avatar seed={p.avatar_seed} size={52} />
                <View className="min-w-0 flex-1 gap-0.5">
                  <PersonaName p={p} />
                  <Counts id={p.id} />
                </View>
                <ChevronRight size={20} color={colors.faint} />
              </Pressable>
              <PersonaActions
                persona={p}
                context={target?.context}
                contextId={target?.contextId}
                onNavigate={close}
              />
            </>
          ) : null}
        </View>
      </Modal>
    </AuthorCtx.Provider>
  );
}
