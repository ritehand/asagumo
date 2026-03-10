package asagumo

import (
	"log"
	"os"
)

var (
	Token   string
	GuildID string
)

func init() {
	if _, ok := os.LookupEnv("DEV"); ok {
		Token = os.Getenv("ASAGUMO_TOKEN_DEV")
	} else {
		Token = os.Getenv("ASAGUMO_TOKEN")
	}
	GuildID = os.Getenv("ASAGUMO_GUILD_ID")
	if Token == "" || GuildID == "" {
		log.Fatalln("Missing environment variables")
	}
}
