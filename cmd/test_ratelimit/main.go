package main

import (
	"fmt"
	"log"
	"net/http"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
)

func main() {
	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}

	// Disable auto-retry to see if we get the 429 error immediately
	// s.ShouldRetryOnRateLimit = false // This might be what's needed

	// Let's check what fields are available in Session
	// We can also try to use a custom HTTP client to log headers?

	log.Printf("Starting stress test for roles...")
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("StressTest-%d", i)
		r, err := s.GuildRoleCreate(bot.GuildID, &discordgo.RoleParams{
			Name: name,
		})
		if err != nil {
			reportRateLimit(err)
			log.Printf("Failed: %v", err)
			break
		}
		log.Printf("Created: %s", r.Name)
		// No sleep to trigger rate limit fast
	}
}

func reportRateLimit(err error) {
	if err == nil {
		return
	}

	restErr, ok := err.(*discordgo.RESTError)
	if !ok || restErr.Response == nil {
		return
	}

	resp := restErr.Response
	log.Printf("Status: %d", resp.StatusCode)
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		log.Printf("!!! RATE LIMITED !!!")
		log.Printf("  Retry-After: %s", retryAfter)
	}
}
