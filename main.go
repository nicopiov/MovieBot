package main

import (
	"context"
	"fmt"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const userMovieFile = "usermovies.json"
const blacklistFile = "blacklist.json"

var (
	guildID            = snowflake.MustParse("1151652511502057512")
	isPollActive       bool
	operatingChannelID snowflake.ID
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Error("Error loading .env file")
	}

	slog.Info("starting moviebot...")
	client, err := disgo.New(os.Getenv("TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentsAll)),
		bot.WithEventListenerFunc(commandListener),
		bot.WithEventListenerFunc(pollListener),
	)
	h := handler.New()
	h.Autocomplete("/deletemovie", handleDeleteMovieAutocomplete)
	client.AddEventListeners(h)

	if err != nil {
		slog.Error("Error creating client: " + err.Error())
		return
	}
	defer client.Close(context.TODO())

	if _, err := client.Rest().SetGuildCommands(client.ApplicationID(), guildID, commands); err != nil {
		slog.Error("Error setting guild commands: " + err.Error())
		return
	}

	if err = client.OpenGateway(context.TODO()); err != nil {
		slog.Error("errors while connecting to gateway: " + err.Error())
		return
	}

	slog.Info("example is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func commandListener(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	if funcCommand, ok := funcCommands[data.CommandName()]; ok {
		if err := funcCommand(event); err != nil {
			slog.Error("error while executing " + data.CommandName() + ": " + err.Error())
			err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("error executing command " + data.CommandName()).SetEphemeral(true).Build())
			if err != nil {
				slog.Error("Error sending response: " + err.Error())
				return
			}
		}
	}
}

func pollListener(event *events.MessageCreate) {
	if event.ChannelID == operatingChannelID && event.Message.Type == discord.MessageTypePollResult {
		isPollActive = false
		if event.Message.MessageReference == nil {
			slog.Info("poll non rilevato nel riferimento al messaggio")
			return
		}

		pollMessage, _ := event.Client().Rest().GetMessage(event.ChannelID, *event.Message.MessageReference.MessageID)
		answerID := winningAnswer(pollMessage)
		winningMovie := getWinningMovie(pollMessage, answerID)

		userMovies, err := loadUserMovies(userMovieFile)
		if err != nil {
			slog.Error("error in pollListener: " + err.Error())
			return
		}
		winningUser := getWinningUser(userMovies, winningMovie)

		if err := updateBlacklist(blacklistFile, winningMovie); err != nil {
			slog.Error("error in pollListener while executing updateBlacklist: " + err.Error())
			return
		}
		if err := removeMovieFromUser(userMovies, winningUser, winningMovie); err != nil {
			slog.Error("error in pollListener while executing removeMovieFromUser: " + err.Error())
			return
		}
		if err := saveUserMovies(userMovieFile, userMovies); err != nil {
			slog.Error("error in pollListener while executing saveUserMovies: " + err.Error())
			return
		}
		_, err = event.Client().Rest().CreateMessage(event.ChannelID, discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("Movie '%s' has been removed from %s's list and added to the blacklist", winningMovie, winningUser)).
			Build())
		slog.Info("Movie " + winningMovie + " has been removed from the list and added to the blacklist")
		if err != nil {
			slog.Error("Error sending message: " + err.Error())
		}
		return
	}
}

func getWinningMovie(pollMessage *discord.Message, answerID int) string {
	var winningMovie string
	for _, ans := range pollMessage.Poll.Answers {
		if *ans.AnswerID == answerID {
			winningMovie = *ans.PollMedia.Text
			break
		}
	}
	return winningMovie
}

func winningAnswer(pollMessage *discord.Message) int {
	var answerID int
	highestCount := 0
	for _, answer := range pollMessage.Poll.Results.AnswerCounts {
		if answer.Count > highestCount {
			highestCount = answer.Count
			answerID = answer.ID
		}
	}
	return answerID
}

func getWinningUser(userMovies UserMovies, winningMovie string) string {
	for userID, movies := range userMovies {
		for _, movie := range movies {
			if movie == winningMovie {
				return userID
			}
		}
	}
	return ""
}

func createEmoji(name string) *discord.PartialEmoji {
	emoji := &discord.PartialEmoji{
		Name: &name,
	}
	return emoji
}

func handleDeleteMovieAutocomplete(e *handler.AutocompleteEvent) error {
	usermovies, _ := loadUserMovies(userMovieFile)
	userFilms := usermovies[e.Member().String()]

	var choices []discord.AutocompleteChoice
	for _, movie := range userFilms {
		choices = append(choices, discord.AutocompleteChoiceString{
			Name:  movie[1 : len(movie)-1],
			Value: movie[1 : len(movie)-1],
		})
	}

	return e.AutocompleteResult(choices)
}
