package main

import (
	"fmt"
	"os"
	"strconv"
)

// AppConfig holds the application configuration
type AppConfig struct {
	IRCServer            string
	IRCPort              int
	IRCNick              string
	IRCUser              string
	IRCName              string
	QNetAuthPass         string
	DiscordToken         string
	BridgeDiscordChannelID string
	BridgeIRCChannel     string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*AppConfig, error) {
	config := &AppConfig{}
	var err error

	config.IRCServer = os.Getenv("SPAWNBOT_IRC_SERVER")
	if config.IRCServer == "" {
		return nil, fmt.Errorf("SPAWNBOT_IRC_SERVER is not set")
	}

	ircPortStr := os.Getenv("SPAWNBOT_IRC_PORT")
	if ircPortStr == "" {
		config.IRCPort = 6667 // Default IRC port
	} else {
		config.IRCPort, err = strconv.Atoi(ircPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SPAWNBOT_IRC_PORT: %w", err)
		}
	}

	config.IRCNick = os.Getenv("SPAWNBOT_IRC_NICK")
	if config.IRCNick == "" {
		config.IRCNick = "SpawnBot" // Default Nick
	}

	config.IRCUser = os.Getenv("SPAWNBOT_IRC_USER")
	if config.IRCUser == "" {
		config.IRCUser = "spawnbot" // Default User
	}

	config.IRCName = os.Getenv("SPAWNBOT_IRC_NAME")
	if config.IRCName == "" {
		config.IRCName = "SpawnBot Go IRC Bot" // Default Name
	}

	config.QNetAuthPass = os.Getenv("SPAWNBOT_QNET_AUTH")
	if config.QNetAuthPass == "" {
		// QNET_AUTH is still supported for backward compatibility
		config.QNetAuthPass = os.Getenv("QNET_AUTH")
		if config.QNetAuthPass == "" {
			return nil, fmt.Errorf("SPAWNBOT_QNET_AUTH or QNET_AUTH is not set")
		}
	}

	config.DiscordToken = os.Getenv("SPAWNBOT_DISCORD_TOKEN")
	if config.DiscordToken == "" {
		// SPAWNBOT_TOKEN is still supported for backward compatibility
		config.DiscordToken = os.Getenv("SPAWNBOT_TOKEN")
		if config.DiscordToken == "" {
			return nil, fmt.Errorf("SPAWNBOT_DISCORD_TOKEN or SPAWNBOT_TOKEN is not set")
		}
	}

	config.BridgeDiscordChannelID = os.Getenv("SPAWNBOT_DISCORD_CHANNEL_ID")
	if config.BridgeDiscordChannelID == "" {
		return nil, fmt.Errorf("SPAWNBOT_DISCORD_CHANNEL_ID is not set")
	}

	config.BridgeIRCChannel = os.Getenv("SPAWNBOT_IRC_CHANNEL")
	if config.BridgeIRCChannel == "" {
		return nil, fmt.Errorf("SPAWNBOT_IRC_CHANNEL is not set")
	}

	return config, nil
}
