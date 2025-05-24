package main

import (
	"context"
	"fmt"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/joho/godotenv"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type function func(event *events.ApplicationCommandInteractionCreate) error

var (
	guildID = snowflake.MustParse("1151652511502057512")

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
			Name:        "closepoll",
			Description: "Close the poll",
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
	}

	isPollActive       bool
	operatingChannelID snowflake.ID
)

func main() {
	err := godotenv.Load()
	slog.Info("starting moviebot...")
	client, err := disgo.New(os.Getenv("TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentsAll)),
		bot.WithEventListenerFunc(commandListener),
		bot.WithEventListenerFunc(pollListener),
	)
	if err != nil {
		slog.Error("Error creating client: ", err)
		return
	}
	defer client.Close(context.TODO())

	if _, err := client.Rest().SetGuildCommands(client.ApplicationID(), guildID, commands); err != nil {
		slog.Error("Error setting guild commands: ", err)
		return
	}

	if err = client.OpenGateway(context.TODO()); err != nil {
		slog.Error("errors while connecting to gateway", slog.Any("err", err))
		return
	}

	slog.Info("example is now running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-s
}

func commandListener(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	if funcCommmand, ok := funcCommands[data.CommandName()]; ok {
		if err := funcCommmand(event); err != nil {
			err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(err.Error()).Build())
			if err != nil {
				event.Client().Logger().Error("Error sending response", slog.Any("err", err))
				return
			}
		}
	}
}

func pollListener(event *events.MessageCreate) {

	if event.ChannelID == operatingChannelID && event.Message.Type == discord.MessageTypePollResult {
		isPollActive = false

		if event.Message.MessageReference == nil {
			event.Client().Logger().Info("poll non rilevato nel riferimento al messaggio")
			return
		}
		pollMessage, _ := event.Client().Rest().GetMessage(event.ChannelID, *event.Message.MessageReference.MessageID)
		highestCount := 0
		answerID := 0

		for _, answer := range pollMessage.Poll.Results.AnswerCounts {
			if answer.Count > highestCount {
				highestCount = answer.Count
				answerID = answer.ID
			}
		}

		userMovies, err := loadUserMovies("usermovies.json")
		if err != nil {
			event.Client().Logger().Error("Error:", slog.Any("err", err))
			return
		}

		var winningMovie string
		for _, ans := range pollMessage.Poll.Answers {
			if *ans.AnswerID == answerID {
				winningMovie = *ans.PollMedia.Text
				break
			}
		}
		for userID, movies := range userMovies {
			for _, movie := range movies {
				if movie == winningMovie {
					// Remove the movie
					if err := removeMovieFromUser(userMovies, userID, winningMovie); err != nil {
						event.Client().Logger().Error("Error removing movie:", slog.Any("err", err))
						continue
					}

					blacklist, err := loadBlacklist("blacklist.json")
					if err != nil {
						event.Client().Logger().Error("Error loading blacklist:", slog.Any("err", err))
						continue
					}

					blacklist = append(blacklist, winningMovie)
					if err := saveBlacklist("blacklist.json", blacklist); err != nil {
						event.Client().Logger().Error("Error saving blacklist:", slog.Any("err", err))
						continue
					}

					if err := saveUserMovies("usermovies.json", userMovies); err != nil {
						event.Client().Logger().Error("Error saving user movies:", slog.Any("err", err))
						continue
					}

					_, err = event.Client().Rest().CreateMessage(event.ChannelID, discord.NewMessageCreateBuilder().
						SetContent(fmt.Sprintf("Movie '%s' has been removed from %s's list and added to the blacklist", winningMovie, userID)).
						Build())
					if err != nil {
						event.Client().Logger().Error("Error sending message:", slog.Any("err", err))
					}
					return
				}
			}
		}

		event.Client().Logger().Info("Winning movie not found in any user's list")
	}
}

func extractMovie(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("You can only run this command in the channel you setup the bot").SetEphemeral(true).Build())
	}

	if isPollActive {
		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("❌ There's already an active poll! Please wait for it to finish.").
			SetEphemeral(true).
			Build())
	}

	userMovies, err := loadUserMovies("usermovies.json")
	if err != nil {
		return err
	}

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	firstPerson, secondPerson := extractPeople(userMovies, r)
	movie1, movie2 := extractPersonMovies(userMovies, firstPerson, secondPerson, r)
	question := fmt.Sprint("Ecco i film estratti per domenica: \n")

	builder := discord.PollCreateBuilder{}
	pool := builder.SetQuestion(question).
		AddAnswer(movie1, createEmoji("1️⃣")).
		AddAnswer(movie2, createEmoji("2️⃣")).SetDuration(1).SetAllowMultiselect(false).Build()

	event.Client().Rest().CreateMessage(event.Channel().ID(), discord.NewMessageCreateBuilder().SetContent(fmt.Sprint("Vote @everyone")).Build())

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetPoll(pool).Build()); err != nil {
		log.Println("error creating poll:", err)
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("Failed to create poll: " + err.Error()).SetEphemeral(true).Build())
	}

	isPollActive = true

	return nil
}

func addMovie(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("You can only run this command in the channel you setup the bot").SetEphemeral(true).Build())
	}

	memberID := event.Member().String()
	userMovies, err := loadUserMovies("usermovies.json")
	if err != nil {
		return err
	}

	optionValue, err := event.SlashCommandInteractionData().Options["movie"].Value.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshalling option value: %w", err)
	}

	if _, ok := userMovies[memberID]; ok && len(userMovies[memberID]) == 2 {
		if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint(memberID, " hai già inserito due film, non puoi inserirne altri")).Build()); err != nil {
			event.Client().Logger().Error("Error sending response", slog.Any("err", err))
			return err
		}
		return nil
	}

	movie := string(optionValue)
	if isMovieInUserMovies(userMovies, movie) {
		if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint(memberID, " il film che hai scelto è già presente nella lista")).Build()); err != nil {
			event.Client().Logger().Error("Error sending response", slog.Any("err", err))
			return err
		}
		return nil
	}

	movies := append(userMovies[memberID], movie)
	userMovies[memberID] = movies

	if err := saveUserMovies("usermovies.json", userMovies); err != nil {
		event.Client().Logger().Error("Error:", slog.Any("err", err))
		return err
	}

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint("movie ", movie, " added by ", memberID)).Build()); err != nil {
		event.Client().Logger().Error("Error sending response", slog.Any("err", err))
		return err
	}
	return nil
}

func listMovies(event *events.ApplicationCommandInteractionCreate) error {
	if event.Channel().ID() != operatingChannelID {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("You can only run this command in the channel you setup the bot").SetEphemeral(true).Build())
	}

	userMovies, err := loadUserMovies("usermovies.json")
	if err != nil {
		return err
	}

	text := fmt.Sprint("Ecco i film proposti per domenica: \n")
	for memberID, movies := range userMovies {
		text += fmt.Sprint("- ", memberID, " ha proposto -> ")
		for _, movie := range movies {
			text += fmt.Sprint(movie, " ")
		}
		text += "\n"
	}

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(text).Build()); err != nil {
		event.Client().Logger().Error("Error sending response", slog.Any("err", err))
	}
	return nil
}

func setupBot(event *events.ApplicationCommandInteractionCreate) error {
	if !event.Member().Permissions.Has(discord.PermissionAdministrator) {
		return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("You don't have the required permissions to run this command").SetEphemeral(true).Build())
	}

	optionValue, err := event.SlashCommandInteractionData().Options["channel"].Value.MarshalJSON()
	channel := strings.ReplaceAll(string(optionValue), "\"", "")
	event.Client().Logger().Info("channel", slog.Any("channel", channel))
	if err != nil {
		return fmt.Errorf("error marshalling option value: %w", err)
	}
	channelID := snowflake.MustParse(channel)
	operatingChannelID = channelID
	return event.CreateMessage(discord.NewMessageCreateBuilder().SetContent(fmt.Sprint("Bot setup in <#", channelID.String(), "> rerun this command to modify the channel")).Build())
}

func createEmoji(name string) *discord.PartialEmoji {
	emoji := &discord.PartialEmoji{
		Name: &name,
	}
	return emoji
}
