package bowls

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

var dailyPrompts = []string{
	"What's one thing that made today's commute bearable?",
	"Worst thing a colleague said this week? Names changed, obviously.",
	"Plans got cancelled. What did you do instead?",
	"Which station has the best chai stall?",
	"What would you tell your boss if you knew you couldn't be fired?",
	"Song that's been on repeat on your ride this week?",
	"Small win of the day. Go.",
	"What's the most Delhi thing you saw today?",
	"If you could skip one meeting forever, which one?",
	"Who else is pretending to work right now?",
	"What do you wish your friends asked you more often?",
	"Best thing you overheard on the metro (no names, no faces)?",
	"What's keeping you up at night lately?",
	"One thing college didn't prepare you for?",
	"Weekend plans or weekend recovery?",
	"Seat or standing near the door: what's your strategy?",
	"What's a hobby you dropped and want back?",
	"Rate your Monday out of 10 and explain.",
	"What's one kind thing a stranger did for you?",
	"If your commute had a theme song, what would it be?",
	"What advice would you give someone starting their first job?",
	"Most relatable office meme you saw this week?",
	"What's one boundary you set recently?",
	"Rain, heat or pollution: which commute is the worst?",
	"Late night thoughts: what's on your mind?",
	"What's the best ₹50 you've spent this week?",
	"Something you're proud of that nobody noticed?",
	"What's the unwritten rule of your metro line?",
	"Friendship that faded: miss it or relieved?",
	"Tomorrow you get the day off. What do you do?",
}

var ist = time.FixedZone("IST", 5*3600+1800)

// SystemPersona returns the per-city "Metro Buddy" persona that authors daily prompts.
func SystemPersona(tx *gorm.DB, city model.City) (uuid.UUID, error) {
	key := "metrobuddy-" + strings.ToLower(city.Code)
	var p model.Persona
	err := tx.Where("city_id = ? AND alias_key = ?", city.ID, key).First(&p).Error
	if err == nil {
		return p.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}
	u := model.User{PhoneHash: secure.SHA256([]byte("system:" + key)), PhoneEnc: []byte("system"), Status: model.UserActive}
	if err := tx.Create(&u).Error; err != nil {
		return uuid.Nil, err
	}
	p = model.Persona{UserID: u.ID, CityID: city.ID, Alias: "Metro Buddy", AliasKey: key, AvatarSeed: "metrobuddysys", Bio: "Daily prompts for your commute.", Intents: model.StringList{}}
	return p.ID, tx.Create(&p).Error
}

// PostDailyPrompts creates today's prompt in every live city bowl; the (city, day) unique key makes reruns no-ops.
func (h *Handlers) PostDailyPrompts(ctx context.Context) error {
	now := h.now()
	day := now.In(ist).Format("2006-01-02")
	text := dailyPrompts[now.In(ist).YearDay()%len(dailyPrompts)]
	var cities []model.City
	if err := h.DB.WithContext(ctx).Where("is_live = ?", true).Find(&cities).Error; err != nil {
		return err
	}
	for _, city := range cities {
		err := h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var n int64
			if err := tx.Model(&model.DailyPrompt{}).Where("city_id = ? AND day = ?", city.ID, day).Count(&n).Error; err != nil || n > 0 {
				return err
			}
			var bowl model.Bowl
			if err := tx.Where("kind = ? AND city_id = ?", model.BowlCity, city.ID).First(&bowl).Error; err != nil {
				return err
			}
			author, err := SystemPersona(tx, city)
			if err != nil {
				return err
			}
			flair := "Daily Prompt"
			post := model.Post{BowlID: bowl.ID, AuthorPersonaID: author, Type: model.PostText, Title: text, Flair: &flair,
				Status: model.ContentVisible, IsPrompt: true, HotRank: hotRank(0, now)}
			post.CreatedAt = now
			if err := tx.Create(&post).Error; err != nil {
				return err
			}
			return tx.Create(&model.DailyPrompt{CityID: city.ID, Day: day, Text: text, PostID: post.ID}).Error
		})
		if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	}
	return nil
}

// pinnedPrompt is today's prompt post for the bowl's city, shown above city-bowl and home feeds.
func (h *Handlers) pinnedPrompt(ctx context.Context, cityID uuid.UUID) (*model.Post, error) {
	day := h.now().In(ist).Format("2006-01-02")
	var dp model.DailyPrompt
	err := h.DB.WithContext(ctx).Where("city_id = ? AND day = ?", cityID, day).First(&dp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p model.Post
	if err := h.DB.WithContext(ctx).First(&p, "id = ? AND status = ?", dp.PostID, model.ContentVisible).Error; err != nil {
		return nil, nil
	}
	return &p, nil
}
