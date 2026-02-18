package main

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
)

var reservedChannelIDs = []string{
	"1473358599811109005",
	"1473358599811109006",
	"1473375081672736838",
	"1473375081672736841",
	"1473434959438938204",
	"1473435709921689863",
	"1473441058082914314",
	"1473441316401844380",
	"1473446369724465293",
}

func main() {
	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}
	s.ShouldRetryOnRateLimit = false

	if err := s.Open(); err != nil {
		log.Fatalln(err)
	}
	defer s.Close()

	channels, err := s.GuildChannels(bot.GuildID)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Found %d channels in guild.", len(channels))

parentloop:
	for _, c := range channels {
		for _, id := range reservedChannelIDs {
			if c.ID == id {
				continue parentloop
			}
		}
		log.Printf("Deleting %s...", c.Name)
		err := retryOnRateLimit(func() error {
			_, err := s.ChannelDelete(c.ID)
			return err
		})
		if err != nil {
			log.Printf("    Failed to delete %s: %v", c.Name, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err := cleanupOldRoles(s); err != nil {
		log.Printf("Failed to cleanup roles: %v", err)
	}
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
		log.Printf("!!! RATE LIMITED !!! Wait Duration: %v. Sleeping...", waitSec)

		time.Sleep(waitSec + 500*time.Millisecond)
	}
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
