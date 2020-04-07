package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"kaepora/internal/back"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	back        *back.Back
	token       string
	dg          *discordgo.Session
	adminUserID string

	closed bool
	closer chan<- struct{}
}

func New(back *back.Back, token string, closer chan<- struct{}) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		back:        back,
		closer:      closer,
		adminUserID: os.Getenv("KAEPORA_ADMIN_USER"),
		token:       token,
		dg:          dg,
	}

	dg.AddHandler(bot.handleMessage)

	return bot, nil
}

func (bot *Bot) Serve() {
	if bot.closed {
		log.Panic("attempted to serve closed bot")
		return
	}

	log.Println("starting Discord bot")

	if err := bot.dg.Open(); err != nil {
		log.Panic(err)
	}
}

func (bot *Bot) Close() error {
	if bot.closed { // don't close twice
		return nil
	}

	log.Println("closing Discord bot")

	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	close(bot.closer)
	bot.closed = true

	if err := bot.dg.Close(); err != nil {
		return err
	}

	return nil
}

func (bot *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	defer func() {
		r := recover()
		if r != nil {
			log.Print("panic: ", r)
		}
	}()

	// Ignore webooks, self, bots, non-commands.
	if m.Author == nil || m.Author.ID == s.State.User.ID ||
		m.Author.Bot || !strings.HasPrefix(m.Content, "!") {
		return
	}

	log.Printf(
		"<%s(%s)@%s#%s> %s",
		m.Author.String(), m.Author.ID,
		m.GuildID, m.ChannelID,
		m.Content,
	)

	out, err := newUserChannelWriter(s, m.Author)
	if err != nil {
		log.Print(err)
	}
	defer func() {
		if err := out.Flush(); err != nil {
			log.Printf("error when sending message: %s", err)
		}
	}()

	if err := bot.dispatch(m.Message, out); err != nil {
		out.Reset()
		fmt.Fprintln(out, "There was an error processing your command.")

		if errors.Is(err, errPublic("")) {
			fmt.Fprintf(out, "```%s\n```\nIf you need help, send `!help`.", err)
		} else {
			fmt.Fprintf(out, "<@%s> will check the logs when he has time.", bot.adminUserID)
		}

		log.Printf("dispatch error: %s", err)
	}

	if err := bot.maybeCleanupMessage(s, m.ChannelID, m.Message.ID); err != nil {
		log.Printf("unable to cleanup message: %s", err)
	}
}

func (bot *Bot) maybeCleanupMessage(s *discordgo.Session, channelID string, messageID string) error {
	channel, err := s.Channel(channelID)
	if err != nil {
		return err
	}

	if channel.Type != discordgo.ChannelTypeGuildText {
		return nil
	}

	if err := s.ChannelMessageDelete(channelID, messageID); err != nil {
		log.Printf("unable to delete message: %s", err)
	}

	return nil
}

func parseCommand(cmd string) (string, []string) {
	parts := strings.Split(cmd, " ")

	switch len(parts) {
	case 0:
		return "", nil
	case 1:
		return parts[0], nil
	default:
		return parts[0], parts[1:]
	}
}

func (bot *Bot) dispatch(m *discordgo.Message, out io.Writer) error {
	command, args := parseCommand(m.Content)

	switch command { // nolint:gocritic, TODO
	case "!help":
		fmt.Fprint(out, help())
		return nil
	case "!dev":
		return bot.dispatchDev(m, args, out)
	case "!games":
		return bot.dispatchGames(m, args, out)
	case "!leagues":
		return bot.dispatchLeagues(m, args, out)
	case "!self":
		return bot.dispatchSelf(m, args, out)
	default:
		return errPublic(fmt.Sprintf("invalid command: %v", m.Content))
	}
}

func (bot *Bot) dispatchDev(m *discordgo.Message, args []string, out io.Writer) error {
	if m.Author.ID != bot.adminUserID {
		return fmt.Errorf("!dev command ran by a non-admin: %v", args)
	}

	if len(args) < 1 {
		return errPublic("need a subcommand")
	}

	switch args[0] { // nolint:gocritic, TODO
	case "down":
		bot.Close()
	case "panic":
		panic("an admin asked me to panic")
	case "error":
		return errPublic("here's your error")
	case "url":
		fmt.Fprintf(
			out,
			"https://discordapp.com/api/oauth2/authorize?client_id=%s&scope=bot&permissions=%d",
			bot.dg.State.User.ID,
			discordgo.PermissionReadMessages|discordgo.PermissionSendMessages|
				discordgo.PermissionEmbedLinks|discordgo.PermissionAttachFiles|
				discordgo.PermissionManageMessages|discordgo.PermissionMentionEveryone,
		)
	}

	return nil
}

func help() string {
	return strings.ReplaceAll(`Available commands:
'''
!games                # list games
!help                 # display this help message
!leagues              # list leagues
!self name NAME       # set your display name to NAME
!self register        # create your account and link it to your Discord account
'''`, "'''", "```")
}