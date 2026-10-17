package studio

import (
	"testing"

	"github.com/livekit/protocol/auth"
)

func TestListenerTokenIsSubscribeOnlyAndNeverRecords(t *testing.T) {
	lk := NewLiveKit("ws://localhost:7880", "devkey", "secret-secret-secret-secret-secret")
	for _, publish := range []bool{false, true} {
		tok, err := lk.Token("ep_1", "p1", "Chai Lover", publish, false)
		if err != nil {
			t.Fatal(err)
		}
		v, err := auth.ParseAPIToken(tok)
		if err != nil {
			t.Fatal(err)
		}
		_, claims, err := v.Verify("secret-secret-secret-secret-secret")
		if err != nil {
			t.Fatal(err)
		}
		g := claims.Video
		if g.GetCanPublish() != publish || !g.GetCanSubscribe() || g.GetCanPublishData() || g.Recorder || g.RoomRecord || g.Room != "ep_1" {
			t.Fatalf("publish=%v grant %+v", publish, g)
		}
	}
}
