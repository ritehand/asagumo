package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	bot "github.com/ritehand/asagumo"
	"github.com/ritehand/asagumo/database"
)

const (
	reasonKickNoRoles = "ロールが設定されていないため、自動的に退会処理されました。"
)

func main() {
	dryRun := flag.Bool("dry", false, "Dry run")
	flag.Parse()

	db, err := database.InitDB()
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

	membersToKick, err := GetMembersWithoutRoles(s, bot.GuildID)
	if err != nil {
		log.Printf("Failed to get members without role: %v\n", err)
	}

	for _, member := range membersToKick {
		log.Printf("Kicking %s (%s) from guild %s\n", member.User.Username, member.User.ID, bot.GuildID)
		if *dryRun {
			continue
		}

		if channel, err := s.UserChannelCreate(member.User.ID); err != nil {
			fmt.Printf("DM channel create failed: %v\n", err)
		} else {
			if _, err := s.ChannelMessageSend(channel.ID, reasonKickNoRoles); err != nil {
				fmt.Printf("DM send failed: %v\n", err)
			}
		}

		if err := s.GuildMemberDeleteWithReason(bot.GuildID, member.User.ID, reasonKickNoRoles); err != nil {
			log.Printf("Failed to kick %s (%s): %v\n", member.User.Username, member.User.ID, err)
		}
	}

	log.Println("kick finished.")
}

func GetMembersWithoutRoles(s *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
	var membersWithoutRoles []*discordgo.Member
	stop := make(chan struct{})
	nonce := time.Now().Format("20060102150405")

	removeHandler := s.AddHandler(func(s *discordgo.Session, chunk *discordgo.GuildMembersChunk) {
		if chunk.Nonce != nonce || chunk.GuildID != guildID {
			return
		}
		guild, err := s.Guild(chunk.GuildID)
		if err != nil {
			return
		}
		ownerID := guild.OwnerID

		for _, m := range chunk.Members {
			if m.User.ID == ownerID {
				continue
			}
			if m.Permissions&discordgo.PermissionAdministrator != 0 {
				continue
			}
			if len(m.Roles) < 1 {
				membersWithoutRoles = append(membersWithoutRoles, m)
			}
		}

		if chunk.ChunkIndex == chunk.ChunkCount-1 {
			close(stop)
		}
	})
	defer removeHandler()

	err := s.RequestGuildMembers(guildID, "", 0, nonce, false)
	if err != nil {
		return nil, err
	}

	select {
	case <-stop:
		return membersWithoutRoles, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("request timed out")
	}
}
