package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
)

func init() {
	log.SetFlags(0)
}

func main() {
	db, err := bot.InitDB()
	if err != nil {
		log.Fatalln("Failed to initialize database:", err)
	}
	defer db.Close()

	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}
	s.Identify.Intents = discordgo.IntentsAll
	s.ShouldRetryOnRateLimit = false

	if err := s.Open(); err != nil {
		log.Fatalln(err)
	}
	defer s.Close()

	log.Println("Bot session opened.")

	if err := applyCategoryPermissions(s); err != nil {
		log.Printf("Failed to apply permissions: %v", err)
	}

	log.Println("Permission application finished.")
}

func applyCategoryPermissions(s *discordgo.Session) error {
	log.Println("Fetching roles...")
	var roles []*discordgo.Role
	err := retryOnRateLimit(func() error {
		var err error
		roles, err = s.GuildRoles(bot.GuildID)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to fetch roles: %w", err)
	}

	existingRoles := make(map[string]*discordgo.Role)
	var everyoneRoleID string
	for _, r := range roles {
		existingRoles[r.Name] = r
		if r.Name == "@everyone" {
			everyoneRoleID = r.ID
		}
	}

	log.Println("Fetching channels...")
	var channels []*discordgo.Channel
	err = retryOnRateLimit(func() error {
		var err error
		channels, err = s.GuildChannels(bot.GuildID)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to fetch channels: %w", err)
	}

	// Prefecture names (from PrefectureColors in prepare/main.go)
	prefectures := []string{
		"北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県",
		"茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "神奈川県", "山梨県",
		"東京都", "新潟県", "富山県", "石川県", "福井県", "長野県", "岐阜県",
		"静岡県", "愛知県", "三重県", "滋賀県", "京都府", "大阪府", "兵庫県",
		"奈良県", "和歌山県", "鳥取県", "島根県", "岡山県", "広島県", "山口県",
		"徳島県", "香川県", "愛媛県", "高知県", "福岡県", "佐賀県", "長崎県",
		"熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県",
	}

	for _, pref := range prefectures {
		// Find category
		var cat *discordgo.Channel
		for _, c := range channels {
			if c.Type == discordgo.ChannelTypeGuildCategory && c.Name == pref {
				cat = c
				break
			}
		}

		if cat == nil {
			log.Printf("Category not found for %s, skipping.", pref)
			continue
		}

		// Find role
		role, ok := existingRoles[pref]
		if !ok {
			log.Printf("Role not found for %s, skipping.", pref)
			continue
		}

		log.Printf("Applying permissions to category: %s", pref)

		// 1. Deny ViewChannel for @everyone
		err = retryOnRateLimit(func() error {
			return s.ChannelPermissionSet(cat.ID, everyoneRoleID, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionViewChannel)
		})
		if err != nil {
			log.Printf("  Failed to deny ViewChannel for @everyone on %s: %v", pref, err)
		}

		// 2. Allow ViewChannel for prefecture role
		err = retryOnRateLimit(func() error {
			return s.ChannelPermissionSet(cat.ID, role.ID, discordgo.PermissionOverwriteTypeRole, discordgo.PermissionViewChannel, 0)
		})
		if err != nil {
			log.Printf("  Failed to allow ViewChannel for %s role on %s: %v", pref, pref, err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func retryOnRateLimit(f func() error) error {
	for {
		err := f()
		if err == nil {
			return nil
		}

		restErr, ok := err.(*discordgo.RESTError)
		if !ok || restErr.Response == nil || restErr.Response.StatusCode != http.StatusTooManyRequests {
			return err
		}

		resp := restErr.Response
		retryAfterStr := resp.Header.Get("Retry-After")
		resetAfterStr := resp.Header.Get("X-RateLimit-Reset-After")

		retryAfter, _ := strconv.ParseFloat(retryAfterStr, 64)
		if retryAfter == 0 {
			retryAfter, _ = strconv.ParseFloat(resetAfterStr, 64)
		}

		if retryAfter == 0 {
			retryAfter = 5
		}

		waitSec := time.Duration(retryAfter * float64(time.Second))
		resetTime := time.Now().Add(waitSec)

		log.Printf("!!! RATE LIMITED !!! Wait %v (Reset at %v)", waitSec, resetTime.Format("15:04:05.000"))

		time.Sleep(waitSec + 500*time.Millisecond)
	}
}
