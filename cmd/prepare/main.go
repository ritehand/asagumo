package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	bot "github.com/1l0/asagumo"

	"github.com/bwmarrin/discordgo"
)

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
	s.Identify.Intents = discordgo.IntentsAll // Use all for now to avoid permission issues
	s.ShouldRetryOnRateLimit = false

	if err := s.Open(); err != nil {
		log.Fatalln(err)
	}
	defer s.Close()

	log.Println("Bot session opened.")

	// 1. Setup Channels
	if err := createChannels(s, db); err != nil {
		log.Printf("Error during channel/role setup: %v", err)
	}

	// 2. Setup Roles (rate limit on Discord is low for creating roles)
	if err := setupRoles(s); err != nil {
		log.Printf("Failed to setup roles: %v", err)
	}

	// 3. Check Remaining Roles
	if err := checkRoles(s); err != nil {
		log.Printf("Failed to check roles: %v", err)
	}
}

var prefectureColors = map[string]int{
	"北海道": 0x1f77b4,
	"青森県": 0xff7f0e, "岩手県": 0xff7f0e, "宮城県": 0xff7f0e, "秋田県": 0xff7f0e, "山形県": 0xff7f0e, "福島県": 0xff7f0e,
	"茨城県": 0x2ca02c, "栃木県": 0x2ca02c, "群馬県": 0x2ca02c, "埼玉県": 0x2ca02c,
	"千葉県": 0xd62728, "神奈川県": 0xd62728, "山梨県": 0xd62728,
	"東京都": 0x9467bd,
	"新潟県": 0x8c564b, "富山県": 0x8c564b, "石川県": 0x8c564b, "福井県": 0x8c564b, "長野県": 0x8c564b,
	"岐阜県": 0xe377c2, "静岡県": 0xe377c2, "愛知県": 0xe377c2, "三重県": 0xe377c2,
	"滋賀県": 0xbcbd22, "京都府": 0xbcbd22, "大阪府": 0xbcbd22, "兵庫県": 0xbcbd22, "奈良県": 0xbcbd22, "和歌山県": 0xbcbd22,
	"鳥取県": 0x17becf, "島根県": 0x17becf, "岡山県": 0x17becf, "広島県": 0x17becf, "山口県": 0x17becf,
	"徳島県": 0x98df8a, "香川県": 0x98df8a, "愛媛県": 0x98df8a, "高知県": 0x98df8a,
	"福岡県": 0xff9896, "佐賀県": 0xff9896, "長崎県": 0xff9896, "熊本県": 0xff9896, "大分県": 0xff9896, "宮崎県": 0xff9896, "鹿児島県": 0xff9896, "沖縄県": 0xff9896,
}

func createChannels(s *discordgo.Session, db *sql.DB) error {
	log.Println("Starting Channel Creation phase...")
	var channels []*discordgo.Channel
	err := retryOnRateLimit(func() error {
		var err error
		channels, err = s.GuildChannels(bot.GuildID)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to fetch channels: %w", err)
	}
	existingChannels := make(map[string]*discordgo.Channel)
	for _, c := range channels {
		switch c.Type {
		case discordgo.ChannelTypeGuildCategory:
			existingChannels["cat_"+c.Name] = c
		case discordgo.ChannelTypeGuildVoice:
			existingChannels["voice_"+c.Name] = c
		case discordgo.ChannelTypeGuildStageVoice:
			existingChannels["stage_"+c.Name] = c
		case discordgo.ChannelTypeGuildText:
			existingChannels["text_"+c.Name] = c
		case discordgo.ChannelTypeGuildForum:
			existingChannels["forum_"+c.Name] = c
		case discordgo.ChannelTypeGuildMedia:
			existingChannels["media_"+c.Name] = c
		case discordgo.ChannelTypeGuildDirectory:
			existingChannels["directory_"+c.Name] = c
		case discordgo.ChannelTypeGuildPublicThread:
			existingChannels["public_thread_"+c.Name] = c
		case discordgo.ChannelTypeGuildPrivateThread:
			existingChannels["private_thread_"+c.Name] = c
		default:
			existingChannels["other_"+c.Name] = c
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 1; i <= 47; i++ {
		var prefName string
		var districtCount int

		err := db.QueryRow("SELECT name, district_count FROM prefectures WHERE id = ?", i).Scan(&prefName, &districtCount)
		if err == nil {
			log.Printf("Using cached data for prefecture ID %d: %s (%d districts)", i, prefName, districtCount)
		} else if err == sql.ErrNoRows {
			prefName, districtCount, err = fetch(client, i)
			if err != nil {
				log.Printf("Failed to fetch data for prefecture ID %d: %v", i, err)
				continue
			}
			_, err = db.Exec("INSERT INTO prefectures (id, name, district_count) VALUES (?, ?, ?)", i, prefName, districtCount)
			if err != nil {
				log.Printf("Failed to cache data for %s: %v", prefName, err)
			}
		} else {
			log.Printf("Database error for ID %d: %v", i, err)
			continue
		}

		fmt.Printf("Processing %s (%d districts)...\n", prefName, districtCount)

		var cat *discordgo.Channel

		// Delete existing category with same name if it exists
		if c, ok := existingChannels["cat_"+prefName]; ok && c.Type == discordgo.ChannelTypeGuildCategory {
			log.Printf("  Skipping existing category %s...", prefName)
			goto skipcategory

		}

		log.Printf("  Creating category %s...", prefName)
		err = retryOnRateLimit(func() error {
			var err error
			cat, err = s.GuildChannelCreate(bot.GuildID, prefName, discordgo.ChannelTypeGuildCategory)
			return err
		})
		if err != nil {
			log.Printf("Failed to create category %s: %v", prefName, err)
		}
	skipcategory:

		// Delete existing text channel with same name if it exists
		if c, ok := existingChannels[prefName]; ok && c.Type == discordgo.ChannelTypeGuildText {
			log.Printf("  Skipping existing channel %s...", prefName)
			goto skiptextchannel
		}

		log.Printf("  Creating channel %s...", prefName)
		err = retryOnRateLimit(func() error {
			_, err = s.GuildChannelCreateComplex(bot.GuildID, discordgo.GuildChannelCreateData{
				Name:     prefName,
				Type:     discordgo.ChannelTypeGuildText,
				ParentID: cat.ID,
			})
			return err
		})
		if err != nil {
			log.Printf("Failed to create channel %s: %v", prefName, err)
		}
		time.Sleep(100 * time.Millisecond)
	skiptextchannel:

		for j := 1; j <= districtCount; j++ {
			distName := fmt.Sprintf("%s%d区", prefName, j)

			// Delete existing channel with same name if it exists
			// Note: We scan all channels because it might be under a different category
			if c, ok := existingChannels[distName]; ok && c.Type == discordgo.ChannelTypeGuildText {
				log.Printf("  Skipping existing channel %s...", distName)
				continue
			}

			log.Printf("  Creating channel %s...", distName)
			err = retryOnRateLimit(func() error {
				_, err = s.GuildChannelCreateComplex(bot.GuildID, discordgo.GuildChannelCreateData{
					Name:     distName,
					Type:     discordgo.ChannelTypeGuildText,
					ParentID: cat.ID,
				})
				return err
			})
			if err != nil {
				log.Printf("Failed to create channel %s: %v", distName, err)
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Delete existing voice channel with same name if it exists
		if c, ok := existingChannels["voice_"+prefName]; ok && c.Type == discordgo.ChannelTypeGuildVoice {
			log.Printf("  Skipping existing voice channel %s...", prefName)
			goto skipvoicechannel
		}

		log.Printf("  Creating channel %s...", prefName)
		err = retryOnRateLimit(func() error {
			_, err = s.GuildChannelCreateComplex(bot.GuildID, discordgo.GuildChannelCreateData{
				Name:     prefName,
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: cat.ID,
			})
			return err
		})
		if err != nil {
			log.Printf("Failed to create channel %s: %v", prefName, err)
		}
	skipvoicechannel:

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func checkRoles(s *discordgo.Session) error {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Checking roles...")
	var roles []*discordgo.Role
	err := retryOnRateLimit(func() error {
		var err error
		roles, err = s.GuildRoles(bot.GuildID)
		return err
	})
	if err != nil {
		return err
	}
	existingRoles := make(map[string]*discordgo.Role)
	for _, r := range roles {
		existingRoles[r.Name] = r
	}
	log.Printf("Current role count: %d", len(roles))

	for pref, color := range prefectureColors {
		if _, ok := existingRoles[pref]; ok {
			continue
		}
		log.Printf("%s: %v", pref, color)
	}
	return nil
}

func setupRoles(s *discordgo.Session) error {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Setting up roles...")
	var roles []*discordgo.Role
	err := retryOnRateLimit(func() error {
		var err error
		roles, err = s.GuildRoles(bot.GuildID)
		return err
	})
	if err != nil {
		return err
	}
	existingRoles := make(map[string]*discordgo.Role)
	for _, r := range roles {
		existingRoles[r.Name] = r
	}
	log.Printf("Current role count: %d", len(roles))

	for pref, color := range prefectureColors {
		if _, ok := existingRoles[pref]; ok {
			continue
		}
		log.Printf("Creating prefecture role: %s...", pref)
		err := retryOnRateLimit(func() error {
			r, err := s.GuildRoleCreate(bot.GuildID, &discordgo.RoleParams{
				Name:        pref,
				Color:       &color,
				Hoist:       val(false),
				Mentionable: val(true),
			})
			if err == nil {
				log.Printf("Successfully created role: %s (ID: %s)", pref, r.ID)
				existingRoles[pref] = r
			}
			return err
		})
		if err != nil {
			log.Printf("Failed to create role %s: %v", pref, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	for i := 1; i <= 30; i++ {
		distNum := fmt.Sprintf("%d区", i)
		if _, ok := existingRoles[distNum]; ok {
			continue
		}
		log.Printf("Creating district role: %s...", distNum)
		err := retryOnRateLimit(func() error {
			r, err := s.GuildRoleCreate(bot.GuildID, &discordgo.RoleParams{
				Name:        distNum,
				Hoist:       val(false),
				Mentionable: val(true),
			})
			if err == nil {
				log.Printf("Successfully created role: %s (ID: %s)", distNum, r.ID)
				existingRoles[distNum] = r
			}
			return err
		})
		if err != nil {
			log.Printf("Failed to create role %s: %v", distNum, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

func val[T any](v T) *T {
	return &v
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

		// Discord provides Retry-After in seconds as a float or int
		retryAfter, _ := strconv.ParseFloat(retryAfterStr, 64)
		if retryAfter == 0 {
			retryAfter, _ = strconv.ParseFloat(resetAfterStr, 64)
		}

		if retryAfter == 0 {
			retryAfter = 5 // Fallback
		}

		waitSec := time.Duration(retryAfter * float64(time.Second))
		resetTime := time.Now().Add(waitSec)

		log.Printf("!!! RATE LIMITED !!!")
		log.Printf("  Wait Duration: %v", waitSec)
		log.Printf("  Expected Reset Time: %v", resetTime.Format("15:04:05.000"))
		log.Printf("  Sleeping until reset...")

		time.Sleep(waitSec + 500*time.Millisecond) // Buffer
		log.Printf("  Resuming...")
	}
}

func fetch(client *http.Client, i int) (string, int, error) {
	url := fmt.Sprintf("https://go2senkyo.com/shugiin/28030/prefecture/%d", i)
	log.Printf("Fetching %s...", url)

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", url, err)
		return "", 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body %s: %v", url, err)
		return "", 0, err
	}
	body := string(bodyBytes)

	prefNameRe := regexp.MustCompile(`＜(.*?)＞`)
	matches := prefNameRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		log.Printf("Failed to find prefecture name for ID %d", i)
		return "", 0, fmt.Errorf("failed to find prefecture name for ID %d", i)
	}
	prefName := matches[1]

	districtRe := regexp.MustCompile(`>([^<]+?([0-9]+区))<`)
	districtMatches := districtRe.FindAllStringSubmatch(body, -1)

	seen := make(map[string]bool)
	districts := []string{}
	for _, m := range districtMatches {
		distName := m[1]
		if !seen[distName] {
			seen[distName] = true
			districts = append(districts, distName)
		}
	}
	districtCount := len(districts)

	return prefName, districtCount, nil
}
