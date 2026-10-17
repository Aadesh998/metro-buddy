package studio

import (
	"context"
	"net/http"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// Live is the slice of LiveKit the studio uses; tests swap in a recorder.
type Live interface {
	URL() string
	Token(room, identity, name string, publish, admin bool) (string, error)
	SetPublish(ctx context.Context, room, identity string, publish bool) error
	Remove(ctx context.Context, room, identity string) error
	CloseRoom(ctx context.Context, room string) error
	StopEgress(ctx context.Context, egressID string) error
	Webhook(r *http.Request) (*livekit.WebhookEvent, error)
}

type LiveKit struct {
	url, key, secret string
	rooms            *lksdk.RoomServiceClient
	egress           *lksdk.EgressClient
}

// NewLiveKit talks to LiveKit Cloud or a self-hosted server; `livekit-server --dev` uses devkey/secret.
func NewLiveKit(url, key, secret string) *LiveKit {
	httpURL := url
	if len(url) > 5 && url[:5] == "wss:/" {
		httpURL = "https:/" + url[5:]
	} else if len(url) > 4 && url[:4] == "ws:/" {
		httpURL = "http:/" + url[4:]
	}
	return &LiveKit{url: url, key: key, secret: secret,
		rooms: lksdk.NewRoomServiceClient(httpURL, key, secret), egress: lksdk.NewEgressClient(httpURL, key, secret)}
}

func (l *LiveKit) URL() string { return l.url }

// Token grants exactly what the role needs. Listeners can subscribe but never publish audio,
// and no token ever carries the recorder grant: shows are never recorded.
func (l *LiveKit) Token(room, identity, name string, publish, admin bool) (string, error) {
	g := &auth.VideoGrant{RoomJoin: true, Room: room, RoomAdmin: admin}
	g.SetCanPublish(publish)
	g.SetCanSubscribe(true)
	g.SetCanPublishData(false)
	return auth.NewAccessToken(l.key, l.secret).SetIdentity(identity).SetName(name).
		SetVideoGrant(g).SetValidFor(4 * time.Hour).ToJWT()
}

func (l *LiveKit) SetPublish(ctx context.Context, room, identity string, publish bool) error {
	_, err := l.rooms.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room: room, Identity: identity,
		Permission: &livekit.ParticipantPermission{CanSubscribe: true, CanPublish: publish},
	})
	return err
}

func (l *LiveKit) Remove(ctx context.Context, room, identity string) error {
	_, err := l.rooms.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room, Identity: identity})
	return err
}

func (l *LiveKit) CloseRoom(ctx context.Context, room string) error {
	_, err := l.rooms.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: room})
	return err
}

func (l *LiveKit) StopEgress(ctx context.Context, id string) error {
	_, err := l.egress.StopEgress(ctx, &livekit.StopEgressRequest{EgressId: id})
	return err
}

func (l *LiveKit) Webhook(r *http.Request) (*livekit.WebhookEvent, error) {
	return webhook.ReceiveWebhookEvent(r, auth.NewSimpleKeyProvider(l.key, l.secret))
}
