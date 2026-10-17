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

export type BowlKind = 'city' | 'line' | 'station' | 'office_hub' | 'college' | 'topic';

export type Bowl = {
  id: UUID;
  slug: string;
  name: string;
  description: string;
  kind: BowlKind;
  visibility: 'public' | 'members_only';
  city_id: UUID | null;
  line: { id: UUID; name: string; color: string } | null;
  station: { id: UUID; name: string } | null;
  member_count: number;
  joined: boolean;
  role: string | null;
  rules_md?: string;
  reason?: string;
};

export type MediaItem = { url?: string; w: number; h: number; pending: boolean };

export type Poll = {
  closes_at: string;
  closed: boolean;
  multi: boolean;
  total: number;
  options: { id: UUID; label: string; votes: number; mine: boolean }[];
};

export type Post = {
  id: UUID;
  bowl: { id: UUID; slug: string; name: string };
  author: PersonaRef;
  is_mine: boolean;
  type: 'text' | 'image' | 'poll';
  title: string;
  body: string;
  flair: string | null;
  media: MediaItem[];
  poll: Poll | null;
  score: number;
  my_vote: -1 | 0 | 1;
  comment_count: number;
  pii_hidden: boolean;
  is_prompt: boolean;
  created_at: string;
  show_support?: boolean;
};

export type Comment = {
  id: UUID;
  post_id: UUID;
  parent_id: UUID | null;
  depth: number;
  author: PersonaRef;
  is_op: boolean;
  is_mine: boolean;
  body: string;
  deleted: boolean;
  score: number;
  my_vote: -1 | 0 | 1;
  pii_hidden: boolean;
  created_at: string;
  show_support?: boolean;
};

export type FeedPage = { items: Post[]; next_cursor: string; pinned?: Post | null };
