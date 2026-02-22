package main

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	bot "github.com/ritehand/asagumo"
)

const (
	TargetChannelID = "1473375081672736838"
)

func main() {
	// Initialize Discord session
	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln("Error creating Discord session:", err)
	}

	log.Println("Creating invite...")

	// Create invite for the specified channel
	invite, err := s.ChannelInviteCreate(TargetChannelID, discordgo.Invite{
		MaxAge:    0,    // Never expires
		MaxUses:   0,    // Unlimited uses
		Temporary: true, // Temporary member
		Unique:    true, // Create a unique invite if possible
	})
	if err != nil {
		log.Fatalln("Error creating invite:", err)
	}

	fmt.Printf("Successfully created invite for channel ID: %s\n", TargetChannelID)
	fmt.Printf("Invite URL: https://discord.gg/%s\n", invite.Code)
}
