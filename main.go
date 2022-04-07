package main

import (
	log "github.com/sirupsen/logrus"

	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var botToken = os.Getenv("BOT_TOKEN")

func init() {
	log.SetFormatter(&log.JSONFormatter{})

	if botToken == "" {
		log.Fatal("BOT_TOKEN is invalid")
	}
}

func main() {

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "ping" {
		err := s.MessageReactionAdd(m.ChannelID, m.ID, "👍")
		if err != nil {
			log.Error(err)
			return
		}
		_, err = s.ChannelMessageSend(m.ChannelID, "Pong!")
		if err != nil {
			log.Error(err)
			return
		}
	}

	// If the message is "pong" reply with "Ping!"
	if m.Content == "pong" {
		_, err := s.ChannelMessageSend(m.ChannelID, "Ping!")
		if err != nil {
			log.Error(err)
			return
		}
	}
}
