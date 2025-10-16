package stockcommand

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/stockbot/internal/util"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

var (
	StockCmd = StockCommand{
		Name:        "stock",
		Description: "Stock interaction command",
	}
)

type StockCommand struct {
	Name        string
	Description string
}

func (s StockCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	switch *sub.SubCommandName {
	case "show":
		showHandler(sub, event)
	case "alert":

	}
}

func (s StockCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "show",
			Description: "show stock information",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "symbol",
					Description: "stock symbol",
					Required:    true,
				},
			},
		},
	}
}

func showHandler(args discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	symbol := strings.ToUpper(args.Options["symbol"].String())
	embeds := getShowEmbed(symbol)

	_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &embeds,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", embeds))
	}
}

func getShowEmbed(symbol string) (embeds []discord.Embed) {
	ticker := yfa.NewTicker(symbol)
	// get the latest PriceData
	info, err := ticker.Info()

	if err != nil {
		slog.Error("Error fetching stock", slog.Any("err", err))
		return
	}

	var embed discord.Embed

	// Handle not found
	if info.LongName == "" {
		embed.Title = "Not found"
		embed.Description = fmt.Sprintf("The stock with symbol: %s couldn't be found", symbol)
		embeds = append(embeds, embed)
		return
	}

	var start, end time.Time
	end = time.Now()
	start = end.AddDate(-1, 0, 0)

	if start.Weekday() == time.Saturday {
		start = start.AddDate(0, 0, -1)
	} else if start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, -2)
	}

	hist, err := ticker.History(yfa.HistoryQuery{
		Start:    start.Format("2006-01-02"),
		End:      fmt.Sprintf("%d", end.Unix()),
		Interval: "1d",
	})
	if err != nil {
		slog.Error("Error getting history", slog.Any("err", err))
	}

	embed.Title = info.LongName
	embed.Fields = append(embed.Fields,
		discord.EmbedField{
			Name:   "Price",
			Value:  fmt.Sprintf("%s %s %.2f", info.Currency, info.CurrencySymbol, info.RegularMarketPrice.Raw),
			Inline: util.Pointer(true),
		},
		discord.EmbedField{
			Name:   "Exchange",
			Value:  info.Exchange,
			Inline: util.Pointer(true),
		},
		discord.EmbedField{},
		discord.EmbedField{
			Name:   "Dialy % Change",
			Value:  info.RegularMarketChangePercent.Fmt,
			Inline: util.Pointer(true),
		},
		util.PeriodChangeEmbed("1wk", hist),
		util.PeriodChangeEmbed("1mo", hist),
		util.PeriodChangeEmbed("1y", hist),
	)
	if info.RegularMarketChangePercent.Raw > 0 {
		embed.Color = 5763719
	} else {
		embed.Color = 15548997
	}

	embeds = append([]discord.Embed{}, embed)
	return
}
