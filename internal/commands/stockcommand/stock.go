package stockcommand

import (
	"fmt"
	"log/slog"
	"strings"

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

func (s StockCommand) ComponentHandler(event *events.ComponentInteractionCreate) {
	if event.Message.Interaction.User.ID != event.Member().User.ID {
		return
	}

	err := event.DeferUpdateMessage()

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	details := strings.Split(event.Data.CustomID(), ";")

	component, file := generateComponent(details[1], details[2])

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Components: &[]discord.LayoutComponent{component},
		Files:      []*discord.File{file},
		Flags:      util.ConfigFile.SetComponentV2Flags(),
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", component))
	}
}

func showHandler(args discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	symbol := strings.ToUpper(args.Options["symbol"].String())
	// embeds := getShowEmbed(symbol)
	component, file := generateComponent(symbol, "1y")

	_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		// Embeds: &embeds,
		Components: &[]discord.LayoutComponent{component},
		Files:      []*discord.File{file},
		Flags:      util.ConfigFile.SetComponentV2Flags(),
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", component))
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

	hist, err := util.FetchHistory(ticker, "1y")
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

func generateComponent(symbol, period string) (component discord.LayoutComponent, file *discord.File) {

	ticker := yfa.NewTicker(symbol)
	// get the latest PriceData
	info, err := ticker.Info()

	if err != nil {
		slog.Error("Error fetching stock", slog.Any("err", err))
		return
	}

	hist, err := util.FetchHistory(ticker, period)

	if err != nil {
		slog.Error("Error fetching history", slog.Any("err", err))
		return
	}

	file = util.GenerateLineChart(hist, info, period)

	var color int
	if info.RegularMarketChangePercent.Raw > 0 {
		color = 5763719
	} else {
		color = 15548997
	}

	component = discord.ContainerComponent{
		AccentColor: color,
		Components: []discord.ContainerSubComponent{
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("# %s", symbol),
			},
			discord.SectionComponent{
				Components: []discord.SectionSubComponent{
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Daily %% Change**\n%s", info.RegularMarketChangePercent.Fmt),
					},
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Weekly %% Change:**\n%s", util.PeriodChange("1wk", hist)),
					},
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Yearly %% Change:**\n%s", util.PeriodChange("1y", hist)),
					},
				},
				Accessory: discord.ThumbnailComponent{
					Media: discord.UnfurledMediaItem{
						URL: fmt.Sprintf("attachment://%s", file.Name),
					},
				},
			},
			discord.ActionRowComponent{
				Components: util.GenerateButtons(
					[]util.Button{
						{
							ID:     fmt.Sprintf("stock;%s;%s", info.Symbol, "1d"),
							Label:  "Daily",
							Active: period == "1d",
						},
						{
							ID:     fmt.Sprintf("stock;%s;%s", info.Symbol, "1wk"),
							Label:  "1 Week",
							Active: period == "1wk",
						},
						{
							ID:     fmt.Sprintf("stock;%s;%s", info.Symbol, "1mo"),
							Label:  "1 Month",
							Active: period == "1mo",
						},
						{
							ID:     fmt.Sprintf("stock;%s;%s", info.Symbol, "3mo"),
							Label:  "3 Month",
							Active: period == "3mo",
						},
						{
							ID:     fmt.Sprintf("stock;%s;%s", info.Symbol, "1y"),
							Label:  "1 Year",
							Active: period == "1y",
						},
					},
				),
			},
		},
	}
	return
}
