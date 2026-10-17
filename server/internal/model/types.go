package model

import "github.com/google/uuid"

// Lists and maps are JSON-serialized into text columns so the same models run on Postgres and SQLite.
type StringList []string
type IDList []uuid.UUID
type JSONMap map[string]any
type BoolMap map[string]bool

type UserStatus string

const (
	UserActive          UserStatus = "active"
	UserSuspended       UserStatus = "suspended"
	UserBanned          UserStatus = "banned"
	UserPendingDeletion UserStatus = "pending_deletion"
	UserDeleted         UserStatus = "deleted"
)

type Gender string

const (
	GenderWoman       Gender = "woman"
	GenderMan         Gender = "man"
	GenderNonBinary   Gender = "non_binary"
	GenderUndisclosed Gender = "prefer_not_to_say"
)

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

type ConsentPurpose string

const (
	ConsentCore          ConsentPurpose = "core_service"
	ConsentTravel        ConsentPurpose = "travel_matching"
	ConsentAI            ConsentPurpose = "ai_processing"
	ConsentAnalytics     ConsentPurpose = "analytics"
	ConsentMarketingPush ConsentPurpose = "marketing_push"
)

var ConsentPurposes = []ConsentPurpose{ConsentCore, ConsentTravel, ConsentAI, ConsentAnalytics, ConsentMarketingPush}

type DMPolicy string

const (
	DMRequests DMPolicy = "requests"
	DMNobody   DMPolicy = "nobody"
)

var PersonaIntents = []string{"vent", "listen", "travel_buddy", "friends", "discuss"}

type BowlKind string

const (
	BowlCity      BowlKind = "city"
	BowlLine      BowlKind = "line"
	BowlStation   BowlKind = "station"
	BowlOfficeHub BowlKind = "office_hub"
	BowlCollege   BowlKind = "college"
	BowlTopic     BowlKind = "topic"
)

type BowlVisibility string

const (
	BowlPublic      BowlVisibility = "public"
	BowlMembersOnly BowlVisibility = "members_only"
)

type BowlRole string

const (
	BowlMemberRole BowlRole = "member"
	BowlModerator  BowlRole = "moderator"
	BowlOwner      BowlRole = "owner"
)

type PostType string

const (
	PostText  PostType = "text"
	PostImage PostType = "image"
	PostPoll  PostType = "poll"
)

type ContentStatus string

const (
	ContentVisible         ContentStatus = "visible"
	ContentPendingReview   ContentStatus = "pending_review"
	ContentRemovedByMod    ContentStatus = "removed_by_mod"
	ContentRemovedByAuthor ContentStatus = "removed_by_author"
	ContentHiddenAuto      ContentStatus = "hidden_auto"
)

type VoteTarget string

const (
	VotePost    VoteTarget = "post"
	VoteComment VoteTarget = "comment"
)

type ConversationKind string

const (
	ConvDM         ConversationKind = "dm"
	ConvGroup      ConversationKind = "group"
	ConvCrew       ConversationKind = "crew"
	ConvTravel     ConversationKind = "travel"
	ConvDiscussion ConversationKind = "discussion"
)

type ConvRole string

const (
	ConvMember ConvRole = "member"
	ConvAdmin  ConvRole = "admin"
	ConvOwner  ConvRole = "owner"
)

type DMContext string

const (
	DMCtxPost    DMContext = "post"
	DMCtxComment DMContext = "comment"
	DMCtxRoom    DMContext = "room"
	DMCtxProfile DMContext = "profile"
	DMCtxEpisode DMContext = "episode"
)

type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestAccepted RequestStatus = "accepted"
	RequestIgnored  RequestStatus = "ignored"
	RequestBlocked  RequestStatus = "blocked"
	RequestExpired  RequestStatus = "expired"
)

type MessageKind string

const (
	MsgText      MessageKind = "text"
	MsgImage     MessageKind = "image"
	MsgSystem    MessageKind = "system"
	MsgMeetPoint MessageKind = "meet_point"
)

type RevealLevel string

const (
	RevealNone      RevealLevel = "none"
	RevealFirstName RevealLevel = "first_name"
	RevealPhoto     RevealLevel = "photo"
	RevealContact   RevealLevel = "contact"
)

var RevealOrder = map[RevealLevel]int{RevealNone: 0, RevealFirstName: 1, RevealPhoto: 2, RevealContact: 3}

type RevealAction string

const (
	RevealGrant  RevealAction = "grant"
	RevealRevoke RevealAction = "revoke"
)

type ContentType string

const (
	CTPersona      ContentType = "persona"
	CTPost         ContentType = "post"
	CTComment      ContentType = "comment"
	CTMessage      ContentType = "message"
	CTConversation ContentType = "conversation"
	CTEpisode      ContentType = "episode"
	CTCallIn       ContentType = "call_in"
	CTTrip         ContentType = "trip"
)

type PIIKind string

const (
	PIIPhone   PIIKind = "phone"
	PIIEmail   PIIKind = "email"
	PIIUPI     PIIKind = "upi"
	PIIAadhaar PIIKind = "aadhaar"
	PIIPAN     PIIKind = "pan"
	PIIHandle  PIIKind = "handle"
	PIIURL     PIIKind = "url"
	PIIAddress PIIKind = "address"
)

type Detector string

const (
	DetectorRegex Detector = "regex"
	DetectorLLM   Detector = "llm"
)

type TravelMood string

const (
	MoodChatty TravelMood = "chatty"
	MoodQuiet  TravelMood = "quiet"
	MoodEither TravelMood = "either"
)

type IntentStatus string

const (
	IntentActive    IntentStatus = "active"
	IntentMatched   IntentStatus = "matched"
	IntentExpired   IntentStatus = "expired"
	IntentCancelled IntentStatus = "cancelled"
)

type MatchSideStatus string

const (
	SidePending  MatchSideStatus = "pending"
	SideAccepted MatchSideStatus = "accepted"
	SideDeclined MatchSideStatus = "declined"
)

type MatchStatus string

const (
	MatchProposed MatchStatus = "proposed"
	MatchAccepted MatchStatus = "accepted"
	MatchDeclined MatchStatus = "declined"
	MatchExpired  MatchStatus = "expired"
)

type TripStatus string

const (
	TripPlanned   TripStatus = "planned"
	TripMet       TripStatus = "met"
	TripCompleted TripStatus = "completed"
	TripNoShow    TripStatus = "no_show"
	TripCancelled TripStatus = "cancelled"
)

type SOSStatus string

const (
	SOSOpen             SOSStatus = "open"
	SOSContactsNotified SOSStatus = "contacts_notified"
	SOSResolved         SOSStatus = "resolved"
	SOSFalseAlarm       SOSStatus = "false_alarm"
)

type HostStatus string

const (
	HostApplied   HostStatus = "applied"
	HostApproved  HostStatus = "approved"
	HostSuspended HostStatus = "suspended"
	HostRejected  HostStatus = "rejected"
)

type EpisodeStatus string

const (
	EpisodeScheduled EpisodeStatus = "scheduled"
	EpisodeLive      EpisodeStatus = "live"
	EpisodeEnded     EpisodeStatus = "ended"
	EpisodeCancelled EpisodeStatus = "cancelled"
)

type StageRole string

const (
	StageHost     StageRole = "host"
	StageCoHost   StageRole = "co_host"
	StageOnAir    StageRole = "on_air"
	StageListener StageRole = "listener"
)

type CallInStatus string

const (
	CallInQueued    CallInStatus = "queued"
	CallInOnAir     CallInStatus = "on_air"
	CallInDone      CallInStatus = "done"
	CallInDropped   CallInStatus = "dropped"
	CallInWithdrawn CallInStatus = "withdrawn"
)

type ReportReason string

var ReportReasons = []string{"harassment", "hate", "sexual", "self_harm", "spam", "doxxing", "threat", "underage", "impersonation", "other"}

type Severity string

const (
	SevLow      Severity = "low"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
	SevCritical Severity = "critical"
)

var SeverityRank = map[Severity]int{SevLow: 0, SevMedium: 1, SevHigh: 2, SevCritical: 3}

type CaseStatus string

const (
	CaseOpen      CaseStatus = "open"
	CaseInReview  CaseStatus = "in_review"
	CaseActioned  CaseStatus = "actioned"
	CaseDismissed CaseStatus = "dismissed"
	CaseEscalated CaseStatus = "escalated"
)

type ModAction string

const (
	ActRemoveContent ModAction = "remove_content"
	ActWarn          ModAction = "warn"
	ActMute24h       ModAction = "mute_24h"
	ActSuspend       ModAction = "suspend"
	ActBan           ModAction = "ban"
	ActRestore       ModAction = "restore"
	ActRevokeHost    ModAction = "revoke_host"
)

type AppealStatus string

const (
	AppealOpen       AppealStatus = "open"
	AppealUpheld     AppealStatus = "upheld"
	AppealOverturned AppealStatus = "overturned"
)

type GrievanceCategory string

var GrievanceCategories = []string{"content", "privacy", "impersonation", "ncii", "other"}

type TrustEventKind string

const (
	TrustAccountAge       TrustEventKind = "account_age"
	TrustVerified         TrustEventKind = "verified"
	TrustReportUpheld     TrustEventKind = "report_upheld"
	TrustReportDismissed  TrustEventKind = "report_dismissed"
	TrustPositiveFeedback TrustEventKind = "positive_feedback"
	TrustNoShow           TrustEventKind = "no_show"
	TrustSafetyFlag       TrustEventKind = "safety_flag"
	TrustModAction        TrustEventKind = "mod_action"
)

type NotificationKind string

const (
	NotifDMRequest    NotificationKind = "dm_request"
	NotifMessage      NotificationKind = "message"
	NotifReply        NotificationKind = "reply"
	NotifMatch        NotificationKind = "match"
	NotifTripReminder NotificationKind = "trip_reminder"
	NotifEpisodeLive  NotificationKind = "episode_live"
	NotifModNotice    NotificationKind = "mod_notice"
	NotifSOS          NotificationKind = "sos"
	NotifHostDecision NotificationKind = "host_decision"
)

type AdminRole string

const (
	AdminModerator AdminRole = "moderator"
	AdminTSLead    AdminRole = "trust_safety_lead"
	AdminAdmin     AdminRole = "admin"
)

var AdminRoleRank = map[AdminRole]int{AdminModerator: 0, AdminTSLead: 1, AdminAdmin: 2}

type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorAdmin  ActorType = "admin"
	ActorSystem ActorType = "system"
	ActorAI     ActorType = "ai"
)

type SecurityEventKind string

const (
	SecOTPFail          SecurityEventKind = "otp_fail"
	SecOTPThrottle      SecurityEventKind = "otp_throttle"
	SecRefreshReuse     SecurityEventKind = "refresh_reuse"
	SecPermissionDenied SecurityEventKind = "permission_denied"
	SecRateLimited      SecurityEventKind = "rate_limited"
	SecVaultAccess      SecurityEventKind = "vault_access"
	SecAdminLoginFail   SecurityEventKind = "admin_login_fail"
)

type AIPurpose string

const (
	AIPIISecondPass AIPurpose = "pii_second_pass"
	AIModTriage     AIPurpose = "moderation_triage"
	AIIcebreaker    AIPurpose = "icebreaker"
	AIHostTopics    AIPurpose = "host_topics"
)
