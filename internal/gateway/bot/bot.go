package bot

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Xausdorf/mattermost-poll/internal/domain"
	"github.com/Xausdorf/mattermost-poll/internal/usecase"
	"github.com/mattermost/mattermost-server/v6/model"
)

const (
	maxRetries            = 5
	pollStartMinArgsCount = 2
	pollVoteArgsCount     = 2
)

type Config struct {
	mmUserName string
	mmTeamName string
	mmToken    string
	mmServer   string
}

func LoadConfig() Config {
	var cfg Config

	cfg.mmUserName = os.Getenv("MM_USERNAME")
	if cfg.mmUserName == "" {
		cfg.mmUserName = "PollingBot"
	}
	cfg.mmTeamName = os.Getenv("MM_TEAM")
	if cfg.mmTeamName == "" {
		cfg.mmTeamName = "PollingBot"
	}
	cfg.mmToken = os.Getenv("MM_TOKEN")
	if cfg.mmToken == "" {
		log.Fatal("Mattermost token is not set")
	}
	cfg.mmServer = os.Getenv("MM_SERVER")
	if cfg.mmServer == "" {
		log.Fatal("Mattermost URL is not set")
	}

	return cfg
}

type PollingBot struct {
	cfg             Config
	client          *model.Client4
	webSocketClient *model.WebSocketClient
	user            *model.User
	team            *model.Team
	pollService     *usecase.Poll
}

func NewPollingBot(cfg Config, pollService *usecase.Poll) *PollingBot {
	var bot PollingBot

	bot.cfg = cfg
	bot.client = model.NewAPIv4Client(bot.cfg.mmServer)
	bot.client.SetToken(bot.cfg.mmToken)

	user, resp, err := bot.client.GetMe("")
	if err != nil {
		log.Fatal("Could not log in")
	}
	log.Printf("Logged in to mattermost: user=%v; resp=%v\n", user, resp)
	bot.user = user

	team, resp, err := bot.client.GetTeamByName(cfg.mmTeamName, "")
	if err != nil {
		log.Fatal("Could not find team")
	}
	log.Printf("Team found: team=%v; resp=%v\n", team, resp)
	bot.team = team

	bot.pollService = pollService

	return &bot
}

func (b *PollingBot) Listen(ctx context.Context) {
	for range maxRetries {
		var err error
		b.webSocketClient, err = model.NewWebSocketClient4(b.cfg.mmServer, b.client.AuthToken)
		if err != nil {
			log.Println("Could not connect mattermost websocket, retrying...")
		}
		log.Println("Mattermost websocket succesfully connected")

		b.webSocketClient.Listen()

		log.Println("Polling Bot listening now")
		for {
			select {
			case event := <-b.webSocketClient.EventChannel:
				go b.handleWebSocketEvent(ctx, event)
			case <-ctx.Done():
				return
			}
		}
	}
	log.Fatal("Could not connect mattermost websocket, max retries exceeded")
}

func (b *PollingBot) Close() {
	if b.webSocketClient != nil {
		log.Println("Closing mattermost websocket connection")
		b.webSocketClient.Close()
	}
}

func (b *PollingBot) handleWebSocketEvent(ctx context.Context, event *model.WebSocketEvent) {
	if event.EventType() != model.WebsocketEventPosted {
		return
	}

	post := &model.Post{}
	eventData, ok := event.GetData()["post"].(string)
	if !ok {
		log.Println("Could not cast event data to string")
		return
	}
	if err := json.Unmarshal([]byte(eventData), &post); err != nil {
		log.Println("Could not unmarshal event to *model.Post")
		return
	}

	if post.UserId == b.user.Id {
		return
	}

	b.handlePost(ctx, post)
}

func (b *PollingBot) handlePost(ctx context.Context, post *model.Post) {
	log.Printf("Handling post: msg=%q; post=%v\n", post.Message, post)

	// CSV reading for splitting a string at spaces, except spaces inside quotation marks.
	r := csv.NewReader(strings.NewReader(post.Message))
	r.Comma = ' '
	tokens, err := r.Read()
	if err != nil {
		log.Printf("Could not split post's message: msg=%q; post=%v\n", post.Message, post)
		return
	}
	if len(tokens) == 0 {
		return
	}

	switch tokens[0] {
	case "/poll_start":
		b.handleStart(ctx, post, tokens[1:])
	case "/poll_vote":
		b.handleVote(ctx, post, tokens[1:])
	case "/poll_results":
		b.handleResults(ctx, post, tokens[1:])
	case "/poll_close":
		b.handleClose(ctx, post, tokens[1:])
	case "/poll_delete":
		b.handleDelete(ctx, post, tokens[1:])
	case "/help":
		b.handleHelp(ctx, post, tokens[1:])
	}
}

func (b *PollingBot) Respond(_ context.Context, post *model.Post, msg string) {
	resp := &model.Post{}
	resp.ChannelId = post.ChannelId
	resp.Message = msg
	resp.RootId = post.Id

	if _, _, err := b.client.CreatePost(resp); err != nil {
		log.Printf("Could not respond to post: msg=%q; post=%v; %v\n", msg, post, err)
	}
}

func (b *PollingBot) handleStart(ctx context.Context, post *model.Post, args []string) {
	// /poll_start [question] "[option1]" "[option2]" ...
	if len(args) < pollStartMinArgsCount {
		b.Respond(ctx, post, "Too few arguments. May be you didn't write the options?")
		return
	}

	options := make([]domain.PollOption, len(args)-1)
	for i := 1; i < len(args); i++ {
		options[i] = *domain.NewPollOption(args[i])
	}

	poll := domain.NewPoll(args[0], options, post.UserId)
	if err := b.pollService.CreatePoll(ctx, poll); err != nil {
		log.Printf("Failed to create poll: %v\n", err)
		b.Respond(ctx, post, "Failed to start poll. Try again")
		return
	}
	log.Printf("Poll succesfully created: %v", poll)

	var msgBuilder strings.Builder
	if err := func() error {
		if _, err := msgBuilder.WriteString("Poll succesfully created!\nID: "); err != nil {
			return err
		}
		if _, err := msgBuilder.WriteString(poll.ID); err != nil {
			return err
		}
		for i, option := range options {
			if _, err := msgBuilder.WriteString(fmt.Sprintf("\n%d. %s", i, option.Text)); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		log.Printf("Failed to build response message: %v", err)
		b.Respond(ctx, post, "Failed to start poll. Try again")
		return
	}

	b.Respond(ctx, post, msgBuilder.String())
}

func (b *PollingBot) handleVote(ctx context.Context, post *model.Post, args []string) {
	// /poll_vote [pollID] [vote]
	if len(args) != pollVoteArgsCount {
		b.Respond(ctx, post, "There must be 2 arguments: poll ID and option's number")
		return
	}

	var err error
	answer := &domain.Answer{}
	answer.UserID = post.UserId
	answer.PollID = args[0]
	answer.Vote, err = strconv.Atoi(args[1])
	if err != nil {
		b.Respond(ctx, post, "Vote must be an integer: option's number")
		return
	}

	if err = b.pollService.AddAnswer(ctx, answer); err != nil {
		if errors.Is(err, usecase.ErrAnswerAlreadyExists) {
			b.Respond(ctx, post, "You have already voted in this poll")
			return
		}
		if errors.Is(err, usecase.ErrPollIsNotActive) {
			b.Respond(ctx, post, "Poll is closed, you can not vote")
			return
		}
		if errors.Is(err, usecase.ErrNoSuchOption) {
			b.Respond(ctx, post, "There are not so many options. Try again")
			return
		}
		log.Printf("Failed to add answer: %v\n", err)
		b.Respond(ctx, post, "Failed to vote in this poll. Try again")
		return
	}

	b.Respond(ctx, post, "Vote successfully registered")
}

func (b *PollingBot) handleResults(ctx context.Context, post *model.Post, args []string) {
	// /poll_results [pollID]
	if len(args) != 1 {
		b.Respond(ctx, post, "There must be 1 argument: poll ID")
		return
	}

	pollID := args[0]
	poll, err := b.pollService.GetPollByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, usecase.ErrPollNotFound) {
			b.Respond(ctx, post, "There is no poll with such ID. Try again")
			return
		}
		log.Printf("Failed to get poll results: %v\n", err)
		b.Respond(ctx, post, "Failed to obtain poll results. Try again")
		return
	}

	var msgBuilder strings.Builder
	if err = func() error {
		if _, err = msgBuilder.WriteString(poll.Question); err != nil {
			return err
		}
		for i, option := range poll.Options {
			if _, err = msgBuilder.WriteString(fmt.Sprintf("\n%d. %s\nVotes: %d", i, option.Text, option.Votes)); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		log.Printf("Failed to build response message: %v", err)
		b.Respond(ctx, post, "Failed to obtain poll results. Try again")
		return
	}

	b.Respond(ctx, post, msgBuilder.String())
}

func (b *PollingBot) handleClose(ctx context.Context, post *model.Post, args []string) {
	// /poll_close [pollID]
	if len(args) != 1 {
		b.Respond(ctx, post, "There must be 1 argument: poll ID")
		return
	}

	pollID := args[0]
	if err := b.pollService.ClosePollByID(ctx, pollID, post.UserId); err != nil {
		if errors.Is(err, usecase.ErrPollNotFound) {
			b.Respond(ctx, post, "Failed to close poll: there is no poll with such ID. Try again")
			return
		}
		if errors.Is(err, usecase.ErrUserIsNotPollAuthor) {
			b.Respond(ctx, post, "You can not close this poll, only author can")
			return
		}
		log.Printf("Failed to close poll: %v\n", err)
		b.Respond(ctx, post, "Failed to close poll. Try again")
		return
	}

	b.Respond(ctx, post, "Poll succesfully closed")
}

func (b *PollingBot) handleDelete(ctx context.Context, post *model.Post, args []string) {
	// /poll_delete [pollID]
	if len(args) != 1 {
		b.Respond(ctx, post, "There must be 1 argument: poll ID")
		return
	}

	pollID := args[0]
	if err := b.pollService.DeletePollByID(ctx, pollID, post.UserId); err != nil {
		if errors.Is(err, usecase.ErrPollNotFound) {
			b.Respond(ctx, post, "Failed to delete poll: there is no poll with such ID. Try again")
			return
		}
		if errors.Is(err, usecase.ErrUserIsNotPollAuthor) {
			b.Respond(ctx, post, "You can not delete this poll, only author can")
			return
		}
		log.Printf("Failed to delete poll: %v\n", err)
		b.Respond(ctx, post, "Failed to delete poll. Try again")
		return
	}

	b.Respond(ctx, post, "Poll succesfully deleted")
}

func (b *PollingBot) handleHelp(ctx context.Context, post *model.Post, _ []string) {
	// /help
	b.Respond(ctx, post, `Available commands:
	* /help - info about commands

	* /poll_start [question] "[option1]" "[option2]" ... - creates a poll and returns poll's ID. 
	IMPORTANT: options must be quoted.

	* /poll_vote [pollID] [vote] - register user's vote. Parameter [vote] is number of option in the list of options.
	
	* /poll_results [pollID] - shows poll's results.
	
	* /poll_close [pollID] - author of poll can close it.
	
	* /poll_delete [pollID] - author of poll can delete it.`)
}
