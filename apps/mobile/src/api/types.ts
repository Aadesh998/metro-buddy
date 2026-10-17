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
  trust_tier: number;
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

export type MediaItem = { id: string; url?: string; w: number; h: number; pending: boolean };

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
  locked: boolean;
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

export type MessageKind = 'text' | 'image' | 'system' | 'meet_point' | 'icebreaker';

export type Message = {
  id: UUID;
  conversation_id: UUID;
  sender: PersonaRef | null;
  is_mine: boolean;
  kind: MessageKind;
  body: string;
  media: MediaItem[];
  reply_to_id: UUID | null;
  deleted: boolean;
  pii_hidden: boolean;
  created_at: string;
  expires_at: string | null;
  show_support?: boolean;
  // Client-only state for optimistic sends.
  local?: 'sending' | 'failed';
};

export type ConversationKind = 'dm' | 'group' | 'crew' | 'travel' | 'discussion';

export type Conversation = {
  id: UUID;
  kind: ConversationKind;
  title: string | null;
  topic?: string;
  members: PersonaRef[];
  member_count: number;
  my_role: 'member' | 'admin' | 'owner';
  last_message: Message | null;
  last_message_at: string | null;
  unread: number;
  muted_until: string | null;
  starts_at?: string;
  ends_at?: string;
  closed: boolean;
  slow_mode_sec: number;
  disappear_after_sec: number | null;
  invite_code?: string;
  bowl_id?: UUID;
  match_id?: UUID;
  crew_id?: UUID;
  reveal_levels?: Record<string, string>;
};

export type DMRequest = {
  id: UUID;
  from: PersonaRef;
  to: PersonaRef;
  intro: string;
  context_type: string;
  status: string;
  conversation_id: UUID | null;
  created_at: string;
  expires_at: string;
};

export type Room = {
  id: UUID;
  title: string;
  topic: string;
  bowl_id: UUID | null;
  starts_at: string;
  ends_at: string;
  live: boolean;
  member_count: number;
  joined: boolean;
  slow_mode_sec: number;
};

export type Notification = { id: UUID; kind: string; payload: Record<string, any>; read: boolean; created_at: string };

export type StationRef = { id: UUID; name: string };

export type CommutePlan = { id: UUID; from: StationRef; to: StationRef; depart_from: string; depart_to: string; weekdays: number[]; stops: number; active: boolean };

export type TravelIntent = { id: UUID; from: StationRef; to: StationRef; depart_at: string; expires_at: string; mood: 'chatty' | 'quiet' | 'either'; status: string; from_plan: boolean };

export type Match = {
  id: UUID;
  other: PersonaRef;
  other_vibes: Vibe[];
  score: number;
  why: { chips: string[]; shared_vibes: string[]; overlap_count: number; minutes_apart: number };
  segment_from: StationRef;
  segment_to: StationRef;
  other_mood: 'chatty' | 'quiet' | 'either';
  my_status: 'pending' | 'accepted' | 'declined';
  they_accepted: boolean;
  status: 'proposed' | 'accepted' | 'declined' | 'expired';
  expires_at: string;
  conversation_id: UUID | null;
  trip_id: UUID | null;
};

export type Trip = {
  id: UUID;
  status: 'planned' | 'met' | 'completed' | 'no_show' | 'cancelled';
  meet_station: StationRef;
  meet_platform: string;
  meet_coach: string;
  meet_at: string;
  participants: { persona: PersonaRef; checked_in: boolean; no_show: boolean; is_me: boolean }[];
  conversation_id: UUID | null;
  shared: boolean;
  share_url?: string;
  feedback_given: boolean;
  match_id: UUID | null;
};

export type TrustedContact = { id: UUID; name: string; phone_masked: string };

export type StageRole = 'host' | 'co_host' | 'on_air' | 'listener';

export type CallIn = {
  id: UUID;
  persona: PersonaRef;
  topic?: string;
  status: 'queued' | 'on_air' | 'done' | 'dropped' | 'withdrawn';
  on_air_at: string | null;
  ends_at: string | null;
  voice_masked: boolean;
  created_at: string;
};

export type Episode = {
  id: UUID;
  show_id: UUID;
  show_title: string;
  host: PersonaRef;
  title: string;
  scheduled_at: string;
  status: 'scheduled' | 'live' | 'ended' | 'cancelled';
  listeners: number;
  peak_listeners: number;
  my_role: StageRole | null;
  on_air: CallIn | null;
  my_call_in: CallIn | null;
  queue_length: number;
  auto_muted: boolean;
};

export type Show = {
  id: UUID;
  host: PersonaRef;
  title: string;
  description: string;
  topic_tags: string[];
  schedule: string | null;
  follower_count: number;
  following: boolean;
  is_mine: boolean;
  next_episode: Episode | null;
};

export type HostApplication = { status: 'applied' | 'approved' | 'suspended' | 'rejected'; tagline: string; created_at: string };

export type EpisodeQuestion = { id: UUID; persona: PersonaRef; text: string; answered: boolean; created_at: string };

export type JoinGrant = { url: string; token: string; room: string; role: StageRole };

export type RevealLevel = 'none' | 'first_name' | 'photo' | 'contact';
export type RevealCard = { first_name: string; photo_url: string | null; contact: string };
export type RevealState = { mine: RevealLevel; theirs: RevealLevel; mutual: RevealLevel; revealed: RevealCard; can_offer: RevealLevel[] };

export type Crew = {
  id: UUID;
  name: string;
  line: { id: UUID; name: string; color: string };
  depart_at: string;
  weekdays: number[];
  members: PersonaRef[];
  pending_invites: number;
  conversation_id: UUID | null;
  is_creator: boolean;
};

export type CrewInvite = { id: UUID; crew: Crew; from: PersonaRef; created_at: string };
