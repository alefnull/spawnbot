package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"spawnbot/cmdhandler"
	"strings"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
	"github.com/lrstanley/girc"
)

// TODO MARK: define Servers / Channels to echo with
// Herobrine's Cave : #general
const BRINE_CHAN_ID = snowflake.ID(1226973379303706768)

// The Spawn : #spawn
const SPAWN_CHAN_ID = snowflake.ID(482513037530497025)

func main() {
	// =============================================================================================
	//   /###### /#######   /######
	//  |_  ##_/| ##__  ## /##__  ##
	//    | ##  | ##  \ ##| ##  \__/
	//    | ##  | #######/| ##
	//    | ##  | ##__  ##| ##
	//    | ##  | ##  \ ##| ##    ##
	//   /######| ##  | ##|  ######/
	//  |______/|__/  |__/ \______/
	// =============================================================================================
	irc_client := girc.New(girc.Config{
		Server: "irc.quakenet.org",
		Port:   6667,
		Nick:   "SpawnBot",
		User:   "SpawnBot",
		Name:   "SpawnBot",
		// Debug:  os.Stdout,
	})

	irc_client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Cmd.Join("#spawn")
		// c.Cmd.Join("#spawnbot")
	})

	cmdHandler, cmd_err := cmdhandler.New("!")

	if cmd_err != nil {
		panic(cmd_err)
	}

	cmdHandler.Add(&cmdhandler.Command{
		Name:    "ping",
		Help:    "Sends a pong reply back to the source.",
		MinArgs: 0,
		Fn: func(c *girc.Client, input *cmdhandler.Input) {
			c.Cmd.Reply(*input.Origin, "pong!")
		},
	})

	cmdHandler.Add(&cmdhandler.Command{
		Name:    "die",
		Help:    "Forces the bot to quit.",
		MinArgs: 0,
		Fn: func(c *girc.Client, input *cmdhandler.Input) {
			c.Quit("as you wish")
		},
	})

	irc_client.Handlers.AddHandler(girc.PRIVMSG, cmdHandler)

	// =============================================================================================
	//   /#######  /######  /######   /######   /######  /#######  /#######
	//  | ##__  ##|_  ##_/ /##__  ## /##__  ## /##__  ##| ##__  ##| ##__  ##
	//  | ##  \ ##  | ##  | ##  \__/| ##  \__/| ##  \ ##| ##  \ ##| ##  \ ##
	//  | ##  | ##  | ##  |  ###### | ##      | ##  | ##| #######/| ##  | ##
	//  | ##  | ##  | ##   \____  ##| ##      | ##  | ##| ##__  ##| ##  | ##
	//  | ##  | ##  | ##   /##  \ ##| ##    ##| ##  | ##| ##  \ ##| ##  | ##
	//  | #######/ /######|  ######/|  ######/|  ######/| ##  | ##| #######/
	//  |_______/ |______/ \______/  \______/  \______/ |__/  |__/|_______/
	// =============================================================================================
	dis_client, dis_err := disgo.New(os.Getenv("SPAWNBOT_TOKEN"),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuildMessages,
				gateway.IntentMessageContent,
			),
		),
	)

	if dis_err != nil {
		panic(dis_err)
	}
	defer dis_client.Close(context.TODO())

	dis_client.AddEventListeners(bot.NewListenerFunc(func(event *events.MessageCreate) {
		if event.Message.Author.Bot {
			return
		}

		unprefixed, _ := strings.CutPrefix(event.Message.Content, "!")
		if unprefixed == "die" {
			irc_client.Quit("as you wish")
			dis_client.Close(context.TODO())
			os.Exit(0)
		}

		var author string = event.Message.Author.Username
		var content string = event.Message.Content

		// if len(event.Message.Attachments) > 0 {
		// 	var atts_string string
		// 	for _, att := range event.Message.Attachments {
		// 		atts_string = fmt.Sprintf("%s %s", atts_string, att.URL)
		// 	}

		// 	content += " " + atts_string
		// }

		// for _, mention := range event.Message.Mentions {
		// 	if strings.Contains(content, mention.ID.String()) {
		// 		content = strings.Replace(content, mention.ID.String(), mention.Username, 1)
		// 	}
		// }

		//         /## /##                 /##          /##
		//        | ##|__/                |  ##        |__/
		//    /####### /##  /#######       \  ##        /##  /######   /#######
		//   /##__  ##| ## /##_____/        \  ##      | ## /##__  ## /##_____/
		//  | ##  | ##| ##|  ######          /##/      | ##| ##  \__/| ##
		//  | ##  | ##| ## \____  ##        /##/       | ##| ##      | ##
		//  |  #######| ## /#######/       /##/        | ##| ##      |  #######
		//   \_______/|__/|_______/       |__/         |__/|__/       \_______/
		message := fmt.Sprintf("[%s (discord)]: %s", author, content)

		irc_client.Cmd.Message("#spawn", message)
		// irc_client.Cmd.Message("#spawnbot", message)
	}))

	//   /##                           /##                /## /##
	//  |__/                          |  ##              | ##|__/
	//   /##  /######   /#######       \  ##         /####### /##  /#######
	//  | ## /##__  ## /##_____/        \  ##       /##__  ##| ## /##_____/
	//  | ##| ##  \__/| ##               /##/      | ##  | ##| ##|  ######
	//  | ##| ##      | ##              /##/       | ##  | ##| ## \____  ##
	//  | ##| ##      |  #######       /##/        |  #######| ## /#######/
	//  |__/|__/       \_______/      |__/          \_______/|__/|_______/
	irc_client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		// slog.Debug(fmt.Sprintf("[%s] %s: %s", e.Params[0], e.Source.Name, e.Last()))
		username := e.Source.Name
		message := e.Last()

		_, dis_err = dis_client.Rest().CreateMessage(SPAWN_CHAN_ID, discord.NewMessageCreateBuilder().SetContent(fmt.Sprintf("[%s (irc)]: %s", username, message)).Build())
		// _, dis_err = dis_client.Rest().CreateMessage(BRINE_CHAN_ID, discord.NewMessageCreateBuilder().SetContent(fmt.Sprintf("[%s (irc)]: %s", username, message)).Build())

		if dis_err != nil {
			slog.Error("errors while sending message to discord", slog.Any("err", dis_err))
		}
	})

	if dis_err = dis_client.OpenGateway(context.TODO()); dis_err != nil {
		slog.Error("errors while connecting to gateway", slog.Any("err", dis_err))
		return
	}

	// =============================================================================================
	//   /#######  /########  /######   /######  /##   /## /##   /## /########  /######  /########
	//  | ##__  ##| ##_____/ /##__  ## /##__  ##| ### | ##| ### | ##| ##_____/ /##__  ##|__  ##__/
	//  | ##  \ ##| ##      | ##  \__/| ##  \ ##| ####| ##| ####| ##| ##      | ##  \__/   | ##
	//  | #######/| #####   | ##      | ##  | ##| ## ## ##| ## ## ##| #####   | ##         | ##
	//  | ##__  ##| ##__/   | ##      | ##  | ##| ##  ####| ##  ####| ##__/   | ##         | ##
	//  | ##  \ ##| ##      | ##    ##| ##  | ##| ##\  ###| ##\  ###| ##      | ##    ##   | ##
	//  | ##  | ##| ########|  ######/|  ######/| ## \  ##| ## \  ##| ########|  ######/   | ##
	//  |__/  |__/|________/ \______/  \______/ |__/  \__/|__/  \__/|________/ \______/    |__/
	// =============================================================================================
	for {
		if err := irc_client.Connect(); err != nil {
			slog.Error(err.Error())

			slog.Info("reconnecting in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			return
		}
	}
}
