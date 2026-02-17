package main

import (
	"log"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
)

func main() {
	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}

	// This will fail to compile if the field doesn't exist
	s.ShouldRetryOnRateLimit = false
	log.Println("ShouldRetryOnRateLimit exists and can be set to false.")
}
