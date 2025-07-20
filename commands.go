package main

import (
	"fmt"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

const warningSetup = "You can only run this command in the channel you setup the bot"
const responseError = "error sending response: %w"

type function func(event *events.ApplicationCommandInteractionCreate) error

var (
	commands = []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "listmovie",
			Description: "List all movies inserted for the weekly watchparty",
		},
		discord.SlashCommandCreate{
			Name:        "addmovie",
			Description: "Add a movie to the weekly watchparty",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "movie",
					Description: "The movie to add",
					Required:    true,
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "extractmovie",
			Description: "Extract two movies from the list randomly and create a pool to decide what film to watch",
		},
		discord.SlashCommandCreate{
			Name:        "deletemovie",
			Description: "Delete the specified movie from the user list",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:         "movie",
					Description:  "The movie to remove",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		discord.SlashCommandCreate{
			Name:        "setup",
			Description: "Setup the bot",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionChannel{
					Name:        "channel",
					Description: "The channel to setup the bot",
					ChannelTypes: []discord.ChannelType{
						discord.ChannelTypeGuildText,
					},
					Required: true,
				},
			},
		},
	}

	funcCommands = map[string]function{
		"listmovie":    listMovies,
		"addmovie":     addMovie,
		"extractmovie": extractMovie,
		"setup":        setupBot,
		"deletemovie":  deleteMovie,
	}
)

func extractMovie(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(warningSetup).SetEphemeral(true).Build())
	}

	if isPollActive {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("❌ There's already an active poll! Please wait for it to finish.").
			SetEphemeral(true).
			Build())
	}

	userMovies, err := loadUserMovies(userMovieFile)
	if err != nil {
		return fmt.Errorf("error loading usermovies: %w", err)
	}

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	firstPerson, secondPerson := extractPeople(userMovies, r)
	movie1, movie2 := extractPersonMovies(userMovies, firstPerson, secondPerson, r)
	question := "Ecco i film estratti per domenica: \n"

	builder := discord.PollCreateBuilder{}
	pool := builder.SetQuestion(question).
		AddAnswer(movie1, createEmoji("1️⃣")).
		AddAnswer(movie2, createEmoji("2️⃣")).SetDuration(24).SetAllowMultiselect(false).Build()

	if _, err := event.Client().Rest().CreateMessage(event.Channel().ID(), discord.NewMessageCreateBuilder().SetContent("Vote @everyone").Build()); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetPoll(pool).Build()); err != nil {
		slog.Error("error while creating poll message: " + err.Error())
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("Failed to create poll: " + err.Error()).SetEphemeral(true).Build())
	}

	isPollActive = true
	slog.Info("Movies extracted and poll created successfully")
	return nil
}

func addMovie(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(warningSetup).SetEphemeral(true).Build())
	}

	memberID := event.Member().String()
	userMovies, err := loadUserMovies(userMovieFile)
	if err != nil {
		return fmt.Errorf("error loading usermovies: %v", err)
	}

	optionValue, err := event.SlashCommandInteractionData().Options["movie"].Value.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshalling option value: %w", err)
	}

	if _, ok := userMovies[memberID]; ok && len(userMovies[memberID]) == 2 {
		slog.Info(event.Member().User.Username + " has already added 2 movies")
		if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint(memberID, " hai già inserito due film, non puoi inserirne altri")).Build()); err != nil {
			return fmt.Errorf(responseError, err)
		}
		return nil
	}

	movie := string(optionValue)
	if isMovieInUserMovies(userMovies, movie) {
		if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint(memberID, " il film che hai scelto è già presente nella lista")).Build()); err != nil {
			return fmt.Errorf(responseError, err)
		}
		return nil
	}

	movies := append(userMovies[memberID], movie)
	userMovies[memberID] = movies

	if err := saveUserMovies(userMovieFile, userMovies); err != nil {
		return fmt.Errorf("error saving usermovies: %w", err)
	}

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint("movie ", movie, " added by ", memberID)).Build()); err != nil {
		return fmt.Errorf(responseError, err)
	}
	slog.Info("Added movie " + movie + " by " + event.Member().String())
	return nil
}

func listMovies(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(warningSetup).SetEphemeral(true).Build())
	}

	userMovies, err := loadUserMovies(userMovieFile)
	if err != nil {
		return fmt.Errorf("error loading usermovies: %v", err)
	}

	text := "Ecco i film proposti per domenica: \n"
	for memberID, movies := range userMovies {
		text += fmt.Sprint("- ", memberID, " ha proposto -> ")
		for _, movie := range movies {
			text += fmt.Sprint(movie, " ")
		}
		text += "\n"
	}

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(text).Build()); err != nil {
		return fmt.Errorf(responseError, err)
	}
	slog.Info("Listed movies by " + event.Member().String())
	return nil
}

func setupBot(event *events.ApplicationCommandInteractionCreate) error {
	if !event.Member().Permissions.Has(discord.PermissionAdministrator) {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("You don't have the required permissions to run this command").SetEphemeral(true).Build())
	}

	optionValue, err := event.SlashCommandInteractionData().Options["channel"].Value.MarshalJSON()
	channel := strings.ReplaceAll(string(optionValue), "\"", "")
	if err != nil {
		return fmt.Errorf("error marshalling option value: %w", err)
	}
	channelID := snowflake.MustParse(channel)
	operatingChannelID = channelID
	slog.Info("Setting up bot in channel " + channelID.String())
	return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint("Bot setup in <#", channelID.String(), "> rerun this command to modify the channel")).Build())
}

func deleteMovie(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(warningSetup).SetEphemeral(true).Build())
	}
	userMovies, err := loadUserMovies(userMovieFile)
	if err != nil {
		return fmt.Errorf("error loading usermovies: %v", err)
	}

	optionValue, err := event.SlashCommandInteractionData().Options["movie"].Value.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshalling option value: %w", err)
	}

	movie := string(optionValue)
	if err := removeMovieFromUser(userMovies, event.User().String(), movie); err != nil {
		return fmt.Errorf("error removing movie from usermovies: %w", err)
	}

	slog.Info("Removed movie from " + event.User().String())
	if err := saveUserMovies(userMovieFile, userMovies); err != nil {
		return fmt.Errorf("error saving usermovies: %w", err)
	}
	return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(event.Member().String() + " removed movie " + movie + " from his list").Build())
}
