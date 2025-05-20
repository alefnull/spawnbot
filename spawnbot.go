package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"spawnbot/cmdhandler"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/lrstanley/girc"
)

// runIRCClient manages the IRC client's connection loop.
func runIRCClient(ctx context.Context, ircClient *girc.Client) {
	for {
		select {
		case <-ctx.Done(): // Check if shutdown is requested
			slog.Info("[IRC] Context cancelled, stopping IRC connection attempts.")
			return
		default:
			slog.Info("[IRC] Connecting to server...")
			if err := ircClient.Connect(); err != nil {
				if ctx.Err() != nil { // Check context after connect attempt
					slog.Info("[IRC] Context cancelled during or after connection attempt.")
					return
				}
				slog.Error("[IRC] Connection error", slog.Any("err", err))
				slog.Info("[IRC] Reconnecting in 30 seconds...")
				// Wait for 30 seconds or until context is cancelled
				select {
				case <-time.After(30 * time.Second):
				case <-ctx.Done():
					slog.Info("[IRC] Context cancelled during reconnect wait.")
					return
				}
			} else {
				// If Connect returns without error, it means it was disconnected (e.g. by Quit)
				// or a critical unrecoverable error occurred.
				// Check context to see if this was an intentional shutdown.
				if ctx.Err() != nil {
					slog.Info("[IRC] Disconnected, context cancelled.")
				} else {
					slog.Info("[IRC] Disconnected. Will attempt to reconnect unless shutdown is triggered.")
					// Add a small delay before attempting to reconnect immediately after a normal disconnect
					select {
					case <-time.After(5 * time.Second):
					case <-ctx.Done():
						slog.Info("[IRC] Context cancelled during post-disconnect wait.")
						return
					}
				}
			}
		}
	}
}

// setupSignalHandling configures handling of OS signals for graceful shutdown.
func setupSignalHandling(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("Received signal, initiating shutdown...", slog.String("signal", sig.String()))
		cancel()
	}()
}

// setupCommandHandlers initializes the command handler and registers commands.
func setupCommandHandlers(cancel context.CancelFunc) (*cmdhandler.CmdHandler, error) {
	cmdHandler, err := cmdhandler.New("!")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize command handler: %w", err)
	}

	cmdHandler.Add(&cmdhandler.Command{
		Name:    "ping",
		Help:    "Sends a pong reply back to the source.",
		MinArgs: 0,
		Fn: func(c *girc.Client, input *cmdhandler.Input) {
			slog.Info("Received 'ping' command from IRC.",
				slog.String("user", input.Source.Name),
				slog.String("origin", input.Origin.String()),
			)
			c.Cmd.Reply(*input.Origin, "pong!")
		},
	})

	cmdHandler.Add(&cmdhandler.Command{
		Name:    "die",
		Help:    "Forces the bot to quit.",
		MinArgs: 0,
		Fn: func(c *girc.Client, input *cmdhandler.Input) {
			slog.Info("Received 'die' command from IRC, initiating shutdown.", slog.String("user", input.Source.Name))
			cancel()
		},
	})
	return cmdHandler, nil
}

// setupIRCHandlersAndClient initializes the IRC client and its event handlers.
func setupIRCHandlersAndClient(cfg *AppConfig, cmdHandler *cmdhandler.CmdHandler, discordClient bot.Client) *girc.Client {
	ircClient := girc.New(girc.Config{
		Server: cfg.IRCServer,
		Port:   cfg.IRCPort,
		Nick:   cfg.IRCNick,
		User:   cfg.IRCUser,
		Name:   cfg.IRCName,
	})

	ircClient.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		slog.Info("[IRC] Successfully connected to IRC server.",
			slog.String("server", c.Config.Server),
			slog.String("nick", c.Config.Nick),
		)
		slog.Info("[IRC] Authenticating with QuakeNet...",
			slog.String("auth_user", cfg.IRCNick), // Using IRCNick for QNet AUTH username
		)
		c.Cmd.Message("q@CServe.quakenet.org", fmt.Sprintf("AUTH %s %s", cfg.IRCNick, cfg.QNetAuthPass))
		// It's common for QuakeNet to confirm successful AUTH via a NOTICE or other means,
		// but setting mode +x is a typical next step. We'll log the action.
		c.Cmd.Mode(cfg.IRCNick, "+x")
		slog.Info("[IRC] QuakeNet AUTH command sent and MODE +x requested.",
			slog.String("nick", cfg.IRCNick),
		)
		time.Sleep(time.Second) // Give server time to process
		c.Cmd.Join(cfg.BridgeIRCChannel)
		slog.Info("[IRC] Joined channel.",
			slog.String("channel", cfg.BridgeIRCChannel),
			slog.String("server", c.Config.Server), // Added server for context
		)
	})

	// Handler for IRC messages to be relayed to Discord
	ircClient.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		cmdHandler.Execute(c, e) // Let command handler process first

		// Ensure the message is from the bridged IRC channel and not by the bot itself, and not a command for the bot
		if len(e.Params) > 0 && e.Params[0] == cfg.BridgeIRCChannel && e.Source.Name != cfg.IRCNick && !strings.HasPrefix(e.Last(), cmdHandler.Prefix) {
			username := e.Source.Name
			content := e.Last() 
			message := fmt.Sprintf("[IRC] %s: %s", username, content)

			bridgeDiscordChannelIDSnowflake, parseErr := snowflake.Parse(cfg.BridgeDiscordChannelID)
			if parseErr != nil {
				slog.Error("Invalid BridgeDiscordChannelID in config for IRC relay", slog.String("id", cfg.BridgeDiscordChannelID), slog.Any("err", parseErr))
				return
			}

			if _, err := discordClient.Rest().CreateMessage(bridgeDiscordChannelIDSnowflake, discord.NewMessageCreateBuilder().SetContent(message).Build()); err != nil {
				slog.Error("[DISCORD] Error sending relayed message to Discord",
					slog.String("source_irc_channel", cfg.BridgeIRCChannel),
					slog.String("irc_user", username),
					slog.String("dest_discord_channel_id", cfg.BridgeDiscordChannelID),
					slog.Any("error", err),
				)
			} else {
				slog.Info("Relayed message from IRC to Discord.",
					slog.String("source_irc_channel", cfg.BridgeIRCChannel),
					slog.String("irc_user", username),
					slog.String("dest_discord_channel_id", cfg.BridgeDiscordChannelID),
					slog.String("message_content", content), // Log the original content for brevity
				)
			}
		}
	})
	return ircClient
}

// setupDiscordHandlersAndClient initializes the Discord client and its event handlers.
// Renamed to registerDiscordHandlers in main, keeping it here for consistency with the search block for now.
// Note: The function name `setupDiscordHandlersAndClient` is used in this diff for historical matching reasons,
// but this function is known as `registerDiscordHandlers` in the `main` function's call.
func setupDiscordHandlersAndClient(cfg *AppConfig, cancel context.CancelFunc, ircClient *girc.Client) (bot.Client, error) {
	discordClient, err := disgo.New(cfg.DiscordToken,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuildMessages,
				gateway.IntentMessageContent,
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord client: %w", err)
	}

	discordClient.AddEventListeners(bot.NewListenerFunc(func(event *events.MessageCreate) {
		if event.Message.Author.Bot {
			return
		}

		bridgeDiscordChannelIDSnowflake, parseErr := snowflake.Parse(cfg.BridgeDiscordChannelID)
		if parseErr != nil {
			slog.Error("Invalid BridgeDiscordChannelID in config", slog.String("id", cfg.BridgeDiscordChannelID), slog.Any("err", parseErr))
			return
		}

		if event.Message.ChannelID == bridgeDiscordChannelIDSnowflake {
			unprefixed, _ := strings.CutPrefix(event.Message.Content, "!")
			if unprefixed == "die" {
				slog.Info("Received 'die' command from Discord, initiating shutdown.", slog.String("user", event.Message.Author.Username))
				cancel()
				return
			}

			// Relay message if not a command (e.g. !die)
			if !strings.HasPrefix(event.Message.Content, "!") {
				author := event.Message.Author.Username
				content := event.Message.Content
				// Specific commented-out attachment and mention handling is confirmed removed.
				message := fmt.Sprintf("[DISCORD] %s: %s", author, content) // Keep full formatted message for IRC

				ircClient.Cmd.Message(cfg.BridgeIRCChannel, message)
				slog.Info("Relayed message from Discord to IRC.",
					slog.String("discord_user", author),
					slog.String("source_discord_channel_id", cfg.BridgeDiscordChannelID),
					slog.String("dest_irc_channel", cfg.BridgeIRCChannel),
					slog.String("message_content", content), // Log the original content
				)
			}
		}
	}))
	return discordClient, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", slog.Any("err", err))
		os.Exit(1)
	}
	slog.Info("Configuration loaded successfully.",
		slog.String("irc_server", cfg.IRCServer),
		slog.Int("irc_port", cfg.IRCPort),
		slog.String("irc_nick", cfg.IRCNick),
		slog.String("irc_user", cfg.IRCUser),
		slog.String("irc_name", cfg.IRCName),
		slog.String("discord_channel_id", cfg.BridgeDiscordChannelID),
		slog.String("irc_channel", cfg.BridgeIRCChannel),
	)
	setupSignalHandling(cancel)

	cmdHandler, err := setupCommandHandlers(cancel)
	if err != nil {
		slog.Error("Failed to setup command handlers", slog.Any("err", err))
		os.Exit(1)
	}

	// Create clients
	ircClient := girc.New(girc.Config{
		Server: cfg.IRCServer, Port: cfg.IRCPort, Nick: cfg.IRCNick, User: cfg.IRCUser, Name: cfg.IRCName,
	})
	discordClient, err := disgo.New(cfg.DiscordToken, bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages, gateway.IntentMessageContent)))
	if err != nil {
		slog.Error("Error creating Discord client instance", slog.Any("err", err))
		os.Exit(1)
	}

	// Register handlers (these functions will use the created clients)
	registerIRCHandlers(ircClient, cfg, cmdHandler, discordClient)
	registerDiscordHandlers(discordClient, cfg, cancel, ircClient)


	slog.Info("[DISCORD] Opening gateway connection...")
	if err = discordClient.OpenGateway(ctx); err != nil {
		slog.Error("[DISCORD] Errors while connecting to gateway", slog.Any("err", err))
		os.Exit(1)
	}
	slog.Info("[DISCORD] Gateway connection established successfully.")

	go runIRCClient(ctx, ircClient)

	<-ctx.Done()

	slog.Info("Shutting down gracefully...")

	slog.Info("[DISCORD] Closing connection...")
	if err := discordClient.Close(context.Background()); err != nil {
		slog.Error("[DISCORD] Error closing connection", slog.Any("err", err))
	} else {
		slog.Info("[DISCORD] Connection closed.")
	}

	slog.Info("[IRC] Quitting connection...")
	ircClient.Quit("Shutting down...")

	slog.Info("Shutdown complete.")
}

// registerIRCHandlers registers IRC event handlers.
func registerIRCHandlers(ircClient *girc.Client, cfg *AppConfig, cmdHandler *cmdhandler.CmdHandler, discordClient bot.Client) {
	ircClient.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Cmd.Message("q@CServe.quakenet.org", fmt.Sprintf("AUTH %s %s", cfg.IRCNick, cfg.QNetAuthPass))
		c.Cmd.Mode(cfg.IRCNick, "+x")
		time.Sleep(time.Second)
		c.Cmd.Join(cfg.BridgeIRCChannel)
		slog.Info("[IRC] Connected to " + c.Config.Server + " and joined " + cfg.BridgeIRCChannel)
	})

	ircClient.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		cmdHandler.Execute(c, e) // Command handler execution

		// Relay logic
		if len(e.Params) > 0 && e.Params[0] == cfg.BridgeIRCChannel && e.Source.Name != cfg.IRCNick && !strings.HasPrefix(e.Last(), cmdHandler.Prefix) {
			username := e.Source.Name
			content := e.Last()
			message := fmt.Sprintf("[IRC] %s: %s", username, content)
			bridgeDiscordChannelIDSnowflake, parseErr := snowflake.Parse(cfg.BridgeDiscordChannelID)
			if parseErr != nil {
				slog.Error("Invalid BridgeDiscordChannelID for IRC relay", slog.String("id", cfg.BridgeDiscordChannelID), slog.Any("err", parseErr))
				return
			}
			if _, err := discordClient.Rest().CreateMessage(bridgeDiscordChannelIDSnowflake, discord.NewMessageCreateBuilder().SetContent(message).Build()); err != nil {
				slog.Error("[DISCORD] Error sending relayed message to Discord", slog.Any("err", err))
			} else {
				slog.Info(fmt.Sprintf("Relayed from IRC %s to Discord: %s", cfg.BridgeIRCChannel, message))
			}
		}
	})
}

// registerDiscordHandlers registers Discord event handlers.
func registerDiscordHandlers(discordClient bot.Client, cfg *AppConfig, cancel context.CancelFunc, ircClient *girc.Client) {
	discordClient.AddEventListeners(bot.NewListenerFunc(func(event *events.MessageCreate) {
		if event.Message.Author.Bot {
			return
		}
		bridgeDiscordChannelIDSnowflake, parseErr := snowflake.Parse(cfg.BridgeDiscordChannelID)
		if parseErr != nil {
			slog.Error("Invalid BridgeDiscordChannelID for Discord relay", slog.String("id", cfg.BridgeDiscordChannelID), slog.Any("err", parseErr))
			return
		}

		if event.Message.ChannelID == bridgeDiscordChannelIDSnowflake {
			if strings.HasPrefix(event.Message.Content, "!") {
				unprefixed, _ := strings.CutPrefix(event.Message.Content, "!")
				if unprefixed == "die" {
					slog.Info("Received 'die' command from Discord, initiating shutdown.", slog.String("user", event.Message.Author.Username))
					cancel()
					return
				}
				// Other Discord specific commands could be handled here if necessary.
			}
			
			// Relay message if not a command (e.g. !die)
			if !strings.HasPrefix(event.Message.Content, "!") {
				author := event.Message.Author.Username
				content := event.Message.Content
				// Specific commented-out attachment and mention handling is confirmed removed.
				message := fmt.Sprintf("[DISCORD] %s: %s", author, content)
				ircClient.Cmd.Message(cfg.BridgeIRCChannel, message)
				slog.Info(fmt.Sprintf("Relayed from Discord to IRC %s: %s", cfg.BridgeIRCChannel, message))
			}
		}
	}))
}
