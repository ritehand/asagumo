package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	bot "github.com/1l0/asagumo"

	"github.com/bwmarrin/discordgo"
)

func init() {
	log.SetFlags(0)
}

func main() {
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

	// 1. Cleanup old roles
	// if err := cleanupOldRoles(s); err != nil {
	// 	log.Printf("Failed to cleanup roles: %v", err)
	// }

	// 2. Setup Roles Phase (Prefectures and District Numbers)
	if err := setupRoles(s); err != nil {
		log.Printf("Failed to setup roles: %v", err)
	}

	// 3. Channel Creation Phase (Scraping)
	// if err := createChannels(s); err != nil {
	// 	log.Printf("Error during channel/role setup: %v", err)
	// }

	log.Println("Initial setup finished. Bot is running.")
	select {}
}

// Region Colors
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

func cleanupOldRoles(s *discordgo.Session) error {
	log.Println("Scanning for old roles to cleanup...")
	var roles []*discordgo.Role
	err := retryOnRateLimit(func() error {
		var err error
		roles, err = s.GuildRoles(bot.GuildID)
		return err
	})
	if err != nil {
		return err
	}
	log.Printf("Found %d roles in guild.", len(roles))

	districtNumOnlyRe := regexp.MustCompile(`^[0-9]+区$`)

	count := 0
	deleted := 0
	for _, r := range roles {
		if strings.HasSuffix(r.Name, "区") && !districtNumOnlyRe.MatchString(r.Name) {
			log.Printf("Deleting old role: %s", r.Name)
			err := retryOnRateLimit(func() error {
				return s.GuildRoleDelete(bot.GuildID, r.ID)
			})
			if err != nil {
				log.Printf("Failed to delete role %s: %v", r.Name, err)
			} else {
				deleted++
			}
			count++
			time.Sleep(100 * time.Millisecond)
		}
	}
	log.Printf("Attempted to delete %d roles. Successfully deleted %d roles.", count, deleted)
	return nil
}

func createChannels(s *discordgo.Session) error {
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
		existingChannels[c.Name] = c
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 1; i <= 47; i++ {
		url := fmt.Sprintf("https://go2senkyo.com/shugiin/28030/prefecture/%d", i)

		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", url, err)
			continue
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read body %s: %v", url, err)
			continue
		}
		body := string(bodyBytes)

		prefNameRe := regexp.MustCompile(`＜(.*?)＞`)
		matches := prefNameRe.FindStringSubmatch(body)
		if len(matches) < 2 {
			log.Printf("Failed to find prefecture name for ID %d", i)
			continue
		}
		prefName := matches[1]

		fmt.Printf("Processing %s...\n", prefName)

		var cat *discordgo.Channel
		if c, ok := existingChannels[prefName]; ok && c.Type == discordgo.ChannelTypeGuildCategory {
			cat = c
		} else {
			log.Printf("  Creating category %s...", prefName)
			err := retryOnRateLimit(func() error {
				var err error
				cat, err = s.GuildChannelCreate(bot.GuildID, prefName, discordgo.ChannelTypeGuildCategory)
				return err
			})
			if err != nil {
				log.Printf("Failed to create category %s: %v", prefName, err)
				continue
			}
			existingChannels[prefName] = cat
		}

		districtRe := regexp.MustCompile(`>([^<]+?([0-9]+区))<`)
		districtMatches := districtRe.FindAllStringSubmatch(body, -1)

		seen := make(map[string]bool)
		for _, m := range districtMatches {
			distName := m[1]
			if seen[distName] {
				continue
			}
			seen[distName] = true

			if _, ok := existingChannels[distName]; !ok {
				log.Printf("  Creating channel %s...", distName)
				err := retryOnRateLimit(func() error {
					var err error
					_, err = s.GuildChannelCreateComplex(bot.GuildID, discordgo.GuildChannelCreateData{
						Name:     distName,
						Type:     discordgo.ChannelTypeGuildText,
						ParentID: cat.ID,
					})
					return err
				})
				if err != nil {
					log.Printf("Failed to create channel %s: %v", distName, err)
				} else {
					// We don't need to update existingChannels here as we don't use it again in this loop
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		time.Sleep(500 * time.Millisecond)
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
