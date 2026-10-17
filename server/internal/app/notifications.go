package app

import (
	"context"

	"github.com/google/uuid"

	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
)

func (a *App) alias(ctx context.Context, id uuid.UUID) string {
	refs, err := identity.LoadRefs(a.DB.WithContext(ctx), []uuid.UUID{id})
	if err != nil {
		return "Someone"
	}
	return refs[id].Alias
}

// wireNotifications connects domain events to the notification service.
func (a *App) wireNotifications() {
	receipts := &chat.Receipts{Service: a.Chat, Redis: a.Redis}
	a.Chat.OnMessage = func(ctx context.Context, conv *model.Conversation, msg *model.Message, recipients []uuid.UUID) {
		if msg.SenderPersonaID == nil {
			return
		}
		from := a.alias(ctx, *msg.SenderPersonaID)
		body := from + " sent you a message"
		if conv.Kind == model.ConvGroup || conv.Kind == model.ConvDiscussion || conv.Kind == model.ConvCrew {
			if conv.Title != nil {
				body = from + " in " + *conv.Title
			}
		}
		for _, r := range recipients {
			if r == *msg.SenderPersonaID || receipts.Online(ctx, r) {
				continue
			}
			var m model.ConversationMember
			if a.DB.WithContext(ctx).Where("conversation_id = ? AND persona_id = ?", conv.ID, r).First(&m).Error != nil {
				continue
			}
			if m.MutedUntil != nil && m.MutedUntil.After(msg.CreatedAt) {
				continue
			}
			a.Notify.Notify(ctx, r, model.NotifMessage, nil, notify.Content{Title: "New message", Body: body, Link: "metrobuddy://chat/" + conv.ID.String()}, false)
		}
	}
	a.Chat.OnRequest = func(ctx context.Context, to, requestID uuid.UUID) {
		var r model.DMRequest
		if a.DB.WithContext(ctx).First(&r, "id = ?", requestID).Error != nil {
			return
		}
		a.Notify.Notify(ctx, to, model.NotifDMRequest, model.JSONMap{"request_id": requestID.String(), "from": a.alias(ctx, r.FromPersonaID)},
			notify.Content{Title: "New message request", Body: a.alias(ctx, r.FromPersonaID) + " wants to chat", Link: "metrobuddy://requests"}, true)
	}
	a.Chat.OnAccepted = func(ctx context.Context, requester, convID uuid.UUID) {
		a.Notify.Notify(ctx, requester, model.NotifDMRequest, model.JSONMap{"conversation_id": convID.String(), "accepted": true},
			notify.Content{Title: "Request accepted", Body: "You can chat now", Link: "metrobuddy://chat/" + convID.String()}, true)
	}
	a.Bowls.OnReply = func(ctx context.Context, to, postID, commentID uuid.UUID) {
		a.Notify.Notify(ctx, to, model.NotifReply, model.JSONMap{"post_id": postID.String(), "comment_id": commentID.String()},
			notify.Content{Title: "New reply", Body: "Someone replied to you", Link: "metrobuddy://post/" + postID.String()}, true)
	}
	a.Admin.NotifyPersona = func(persona uuid.UUID, kind model.NotificationKind, payload model.JSONMap) {
		content := notify.Content{Title: "A note from Metro Buddy", Body: "There's an update about your account. Tap for details.", Link: "metrobuddy://notices"}
		if kind == model.NotifHostDecision {
			content = notify.Content{Title: "Vent Studio", Body: "Your host application was reviewed. Tap to see the decision.", Link: "metrobuddy://studio"}
		}
		a.Notify.Notify(context.Background(), persona, kind, payload, content, true)
	}
}
