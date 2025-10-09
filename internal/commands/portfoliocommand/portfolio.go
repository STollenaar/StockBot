package portfoliocommand

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util"
)

var (
	PortfolioCmd = PortfolioCommand{
		Name:        "portfolio",
		Description: "Portfolio interaction command",
	}
)

type PortfolioCommand struct {
	Name        string
	Description string
}

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			now := time.Now()
			if now.Weekday() != time.Saturday && now.Weekday() != time.Sunday {
				// CheckAlerts()
			}
		}
	}()
}

func (s PortfolioCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	switch *sub.SubCommandName {
	case "add":
		addHandler(sub, event)
	case "show":
		showHandler(event)
	case "update":
		addHandler(sub, event)
	case "remove":
		removeHandler(sub, event)
	}
}

func addHandler(args discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	portfolio := database.Portfolio{
		UserID: event.User().ID.String(),
		Symbol: args.Options["symbol"].String(),
		Shares: args.Options["amount"].Float(),
	}

	err := portfolio.UpsertPortfolio()

	response := "Successfully added the stock to your portfolio"

	if err != nil {
		slog.Error("Error adding the stock:", slog.Any("err", err))
		response = "error adding the stock"
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func showHandler(event *events.ApplicationCommandInteractionCreate) {
	portfolios, err := database.GetPortfolio(event.User().ID.String())

	if err != nil {
		slog.Error("Error fetching portfolio:", slog.Any("err", err))
	}

	var embed discord.Embed

	embed.Title = fmt.Sprintf("%s Portfolio", event.User().Username)

	if len(portfolios) == 0 {
		embed.Description = "No stock in your portfolio yet"

		_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed},
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", embed))
		}
		return
	}

	for _, portfolio := range portfolios {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:  portfolio.Symbol,
			Value: fmt.Sprintf("%.2f", portfolio.Shares),
		})
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", embed))
	}
}

func removeHandler(args discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	err := database.RemovePortfolio(event.User().ID.String(), strings.ToUpper(args.Options["symbol"].String()))
	response := "Successfully removed the stock from the portfolio"

	if err != nil {
		slog.Error("Error deleting the portfoliolist:", slog.Any("err", err))
		response = "error removing the stock from the portfolio"
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (s PortfolioCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "add",
			Description: "add a stock to your portfolio",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "symbol",
					Description: "stock symbol",
					Required:    true,
				},
				discord.ApplicationCommandOptionFloat{
					Name:        "amount",
					Description: "Number of shares you have",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "show",
			Description: "show your portfolio",
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "update",
			Description: "update a stock in your portfolio",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "symbol",
					Description: "stock symbol",
					Required:    true,
				},
				discord.ApplicationCommandOptionFloat{
					Name:        "amount",
					Description: "Number of shares you have",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "remove",
			Description: "remove a stock from your portfolio",
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
