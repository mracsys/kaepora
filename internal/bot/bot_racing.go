package bot

import (
	"fmt"
	"io"
	"kaepora/internal/back"
	"kaepora/internal/util"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (bot *Bot) cmdJoin(m *discordgo.Message, args []string, w io.Writer) error {
	player, err := bot.back.GetPlayerByDiscordID(m.Author.ID)
	if err != nil {
		return err
	}

	shortcode := argsAsName(args)
	if shortcode == "" {
		return util.ErrPublic(
			"you need to give the short name of a league so I can know where to add you, " +
				"so see the leagues try `!leagues`",
		)
	}

	session, league, err := bot.back.JoinCurrentMatchSessionByShortcode(player, shortcode)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "You have been registered for the next race in the %s league.\n", league.Name)
	fmt.Fprint(w, "Please ensure you have read the rules before the race: <https://ootrladder.com/en/rules>\n")

	cancelDelta := time.Until(session.StartDate.Time().Add(back.MatchSessionPreparationOffset))
	if cancelDelta > 0 {
		fmt.Fprintf(
			w,
			"If you wish to `!cancel` you have %s to do so, after that you will have to `!forfeit`.",
			cancelDelta.Truncate(time.Second),
		)
	} else { // maybe unreachable, maybe not.
		raceDelta := time.Until(session.StartDate.Time())
		fmt.Fprintf(w,
			"The race begins in %s, you will soon receive your _seed_ details.",
			raceDelta.Truncate(time.Second),
		)
	}

	return nil
}

func (bot *Bot) cmdCancel(m *discordgo.Message, _ []string, w io.Writer) error {
	player, err := bot.back.GetPlayerByDiscordID(m.Author.ID)
	if err != nil {
		return err
	}

	if _, err := bot.back.CancelActiveMatchSession(player.ID); err != nil {
		return err
	}

	fmt.Fprint(w, `You have cancelled your participation for the next race.
This _will not_ count as a loss and won't affect your rankings.`)

	return nil
}

func (bot *Bot) cmdComplete(m *discordgo.Message, _ []string, w io.Writer) error {
	player, err := bot.back.GetPlayerByDiscordID(m.Author.ID)
	if err != nil {
		return err
	}

	if _, err := bot.back.CompleteActiveMatch(player); err != nil {
		return err
	}

	return nil
}

func (bot *Bot) cmdForfeit(m *discordgo.Message, _ []string, w io.Writer) error {
	player, err := bot.back.GetPlayerByDiscordID(m.Author.ID)
	if err != nil {
		return err
	}

	if _, err := bot.back.ForfeitActiveMatch(player); err != nil {
		return err
	}

	return nil
}
