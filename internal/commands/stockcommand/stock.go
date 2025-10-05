package stockcommand

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	yfa "github.com/oscarli916/yahoo-finance-api"
	"github.com/stollenaar/stockbot/internal/util"
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
	err := event.DeferCreateMessage(true)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	var components []discord.LayoutComponent
	var embeds []discord.Embed
	switch *sub.SubCommandName {
	case "show":
		embeds = showHandler(sub)
	}
	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &embeds,
		// Flags:  util.ConfigFile.SetComponentV2Flags(),
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", components))
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

func showHandler(args discord.SlashCommandInteractionData) (embeds []discord.Embed) {
	symbol := strings.ToUpper(args.Options["symbol"].String())

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

	embed.Title = info.LongName
	embed.Fields = append(embed.Fields,
		discord.EmbedField{
			Name:   "Price",
			Value:  fmt.Sprintf("%s %s %.2f", info.Currency, info.CurrencySymbol, info.RegularMarketPrice.Raw),
			Inline: util.Pointer(true),
		},

		discord.EmbedField{
			Name:  "Dialy Change",
			Value: info.RegularMarketChangePercent.Fmt,
		},
	)
	if info.RegularMarketChangePercent.Raw > 0 {
		embed.Color = 5763719
	} else {
		embed.Color = 15548997
	}
	embeds = append(embeds, embed)
	return

	// var container discord.ContainerComponent
	// container.Components = append(container.Components,
	// 	discord.TextDisplayComponent{
	// 		Content: fmt.Sprintf("**Name:** %s\n**Maker Price:** %.2f\n**Market Change:** (%.2f%%)", info.LongName, info.RegularMarketPrice.Raw, info.RegularMarketChangePercent.Raw),
	// 	})

	// components = append(components, container)
}
