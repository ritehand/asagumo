package main

import (
	"fmt"
	"log"

	bot "github.com/1l0/asagumo"
	"github.com/bwmarrin/discordgo"
)

func main() {
	s, err := discordgo.New("Bot " + bot.Token)
	if err != nil {
		log.Fatalln(err)
	}

	if err := s.Open(); err != nil {
		log.Fatalln(err)
	}
	defer s.Close()

	channels, err := s.GuildChannels(bot.GuildID)
	if err != nil {
		log.Fatalln(err)
	}

	nameMap := make(map[string][]string)
	for _, c := range channels {
		nameMap[c.Name] = append(nameMap[c.Name], c.ID)
	}

	duplicatesFound := false
	for name, ids := range nameMap {
		if len(ids) > 1 {
			duplicatesFound = true
			fmt.Printf("Duplicate channel name found: %s\n", name)
			for _, id := range ids {
				fmt.Printf("  - ID: %s\n", id)
			}
		}
	}

	if !duplicatesFound {
		fmt.Println("No duplicate channel names found.")
	}
}
