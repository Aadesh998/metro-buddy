export type UUID = string;

export type Page<T> = { items: T[]; next_cursor?: string };

export type Me = {
  status: 'active' | 'suspended' | 'banned' | 'pending_deletion' | 'deleted';
  has_persona: boolean;
  locale: 'en' | 'hi';
  gender: 'woman' | 'man' | 'non_binary' | 'prefer_not_to_say' | null;
  age_verified: boolean;
  onboarded: boolean;
  core_consent: boolean;
  verified: boolean;
  policy_version: string;
  deletion_scheduled_at: string | null;
};

export type Vibe = { slug: string; label_en: string; label_hi: string; category: string };

export type PersonaRef = { id: UUID; alias: string; avatar_seed: string; verified: boolean; is_host: boolean };

export type PublicPersona = {
  id: UUID;
  alias: string;
  avatar_seed: string;
  bio: string;
  intents: string[];
  is_host: boolean;
  verified: boolean;
  vibes: Vibe[];
  deleted: boolean;
};

export type OwnPersona = Omit<PublicPersona, 'deleted'> & {
  city_id: UUID;
  home_station_id: UUID | null;
  work_station_id: UUID | null;
  alias_change_available_at: string;
};

export type City = { id: UUID; name: string; code: string; is_live: boolean };
export type LineChip = { id: UUID; name: string; color: string };
export type Station = { id: UUID; name: string; is_interchange: boolean; lines: LineChip[]; search_text: string };

export type ConsentPurpose = 'core_service' | 'travel_matching' | 'ai_processing' | 'analytics' | 'marketing_push';
export type ConsentState = { purpose: ConsentPurpose; granted: boolean; policy_version: string; updated_at: string | null };

export type ContentType = 'persona' | 'post' | 'comment' | 'message' | 'conversation' | 'episode' | 'call_in' | 'trip';
export type ReportReason =
  | 'harassment'
  | 'hate'
  | 'sexual'
  | 'self_harm'
  | 'spam'
  | 'doxxing'
  | 'threat'
  | 'underage'
  | 'impersonation'
  | 'other';
