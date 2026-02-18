package main

import (
	"log"

	bot "github.com/1l0/asagumo"

	_ "github.com/mattn/go-sqlite3"
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

	// TODO: implement
}
