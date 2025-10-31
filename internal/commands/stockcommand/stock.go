package stockcommand

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/stockbot/internal/util"
	"github.com/stollenaar/stockbot/internal/util/trackers"
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

func generateComponent(symbol, period string) (component discord.LayoutComponent, file *discord.File) {

	ticker := yfa.NewTicker(symbol)
	// get the latest PriceData
	info, err := ticker.Info()

	if err != nil {
		slog.Error("Error fetching stock", slog.Any("err", err))
		return
	}

	hist, err := trackers.FetchHistory(ticker, period)

	if err != nil {
		slog.Error("Error fetching history", slog.Any("err", err))
		return
	}

	if period == "1d" {
		file = trackers.GenerateLineChart(hist.Daily, info, period)
	}else {
		file = trackers.GenerateLineChart(hist.Yearly, info, period)
	}

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
				Content: fmt.Sprintf("# %s\n%s", symbol, info.LongName),
			},
			discord.SeparatorComponent{
				Divider: util.Pointer(true),
			},
			discord.SectionComponent{
				Components: []discord.SectionSubComponent{
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Price:**\n%s%s %s", info.CurrencySymbol, info.RegularMarketPrice.Fmt, info.Currency),
					},
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Daily %% Change**\n%s\n**Weekly %% Change:**\n%s\n**Yearly %% Change:**\n%s", info.RegularMarketChangePercent.Fmt, trackers.PeriodChange("1wk", hist.Yearly), trackers.PeriodChange("1y", hist.Yearly)),
					},
				},
				Accessory: discord.ThumbnailComponent{
					Media: discord.UnfurledMediaItem{
						URL: fmt.Sprintf("attachment://%s", file.Name),
					},
				},
			},
			discord.SeparatorComponent{
				Divider: util.Pointer(true),
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
