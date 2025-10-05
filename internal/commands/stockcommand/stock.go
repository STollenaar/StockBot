package stockcommand

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/stockbot/internal/util"
)

var (
	StockCmd = StockCommand{
		Name:        "stock",
		Description: "Stock interaction command",
	}
	finnhubClient *finnhub.DefaultApiService
)

type StockCommand struct {
	Name        string
	Description string
}

func init() {
	cfg := finnhub.NewConfiguration()
	token, err := util.GetFinnhub()

	if err != nil {
		log.Fatal(err)
	}

	cfg.AddDefaultHeader("X-Finnhub-Token", token)
	finnhubClient = finnhub.NewAPIClient(cfg).DefaultApi
}

func (s StockCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(true)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	var components []discord.LayoutComponent
	switch *sub.SubCommandName {
	case "show":
		components = showHandler(sub)
	}
	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Components: &components,
		Flags:      util.ConfigFile.SetComponentV2Flags(),
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

func showHandler(args discord.SlashCommandInteractionData) (components []discord.LayoutComponent) {
	quote, _, err := finnhubClient.Quote(context.TODO()).Symbol(args.Options["symbol"].String()).Execute()
	if err != nil {
		slog.Error("Error fetching stock", slog.Any("err", err))
		return
	}

	var container discord.ContainerComponent
	container.Components = append(container.Components,
		discord.TextDisplayComponent{
			Content: fmt.Sprintf("**Name:** %s\n**Maker Price:** %.2f\n**Market Change:** (%.2f%%)", args.Options["symbol"].String(), quote.GetC(), quote.GetDp()),
		})

	components = append(components, container)
	return
}
