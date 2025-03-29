package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Xausdorf/mattermost-poll/internal/gateway/bot"
	"github.com/Xausdorf/mattermost-poll/internal/repository/ttadapter"
	"github.com/Xausdorf/mattermost-poll/internal/usecase"
	"github.com/tarantool/go-tarantool/v2"
	_ "github.com/tarantool/go-tarantool/v2/datetime"
	_ "github.com/tarantool/go-tarantool/v2/decimal"
	_ "github.com/tarantool/go-tarantool/v2/uuid"
)

type tarantoolConfig struct {
	address  string
	user     string
	password string
}

const (
	ttReconnectSeconds = 3
	ttMaxRecconects    = 5
)

func main() {
	ctx := context.Background()

	ttCfg := loadTarantoolConfig()
	conn, err := connectTarantool(ctx, ttCfg)
	if err != nil {
		log.Fatalf("Connection to tarantool refused: %v", err)
	}
	log.Println("Succesfully connected to tarantool")

	pollRepo := ttadapter.NewPollRepository(conn)
	answerRepo := ttadapter.NewAnswerRepository(conn)

	pollService := usecase.NewPoll(pollRepo, answerRepo)

	botConfig := bot.LoadConfig()
	pollingBot := bot.NewPollingBot(botConfig, pollService)
	setupGracefulShutdown(pollingBot)

	pollingBot.Listen(ctx)
}

func setupGracefulShutdown(bot *bot.PollingBot) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			bot.Close()
			log.Println("Shutting down")
			os.Exit(0)
		}
	}()
}

func loadTarantoolConfig() tarantoolConfig {
	var cfg tarantoolConfig

	cfg.address = os.Getenv("TT_ADDRESS")
	if cfg.address == "" {
		cfg.address = "127.0.0.1:3301"
	}
	cfg.user = os.Getenv("TT_USER")
	if cfg.user == "" {
		log.Fatal("Tarantool user is not set")
	}
	cfg.password = os.Getenv("TT_PASSWORD")
	if cfg.password == "" {
		log.Fatal("Tarantool password is not set")
	}

	return cfg
}

func connectTarantool(ctx context.Context, cfg tarantoolConfig) (*tarantool.Connection, error) {
	dialer := tarantool.NetDialer{
		Address:  cfg.address,
		User:     cfg.user,
		Password: cfg.password,
	}
	opts := tarantool.Opts{
		Timeout:       time.Second,
		Reconnect:     ttReconnectSeconds * time.Second,
		MaxReconnects: ttMaxRecconects,
	}

	return tarantool.Connect(ctx, dialer, opts)
}
