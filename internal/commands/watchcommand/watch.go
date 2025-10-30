package watchcommand

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util"
)

var (
	WatchCmd = WatchCommand{
		Name:        "watch",
		Description: "Watch interaction command",
	}
)

type WatchCommand struct {
	Name        string
	Description string
}

func (s WatchCommand) Handler(event *events.ApplicationCommandInteractionCreate) {
	err := event.DeferCreateMessage(util.ConfigFile.SetEphemeral() == discord.MessageFlagEphemeral)

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	sub := event.SlashCommandInteractionData()

	switch *sub.SubCommandName {
	case "add":
		addHandler(sub, event)
	case "list":
		listHandler(event)
	case "update":
		addHandler(sub, event)
	case "remove":
		removeHandler(sub, event)
	}
}

func addHandler(args discord.SlashCommandInteractionData, event *events.ApplicationCommandInteractionCreate) {
	watchList := database.WatchList{
		UserID:      event.User().ID.String(),
		Symbol:      strings.ToUpper(args.Options["symbol"].String()),
		PriceTarget: args.Options["price"].Float(),
		Direction:   args.Options["above"].Bool(),
	}

	err := watchList.UpsertWatchlist()

	response := "Successfully added the watched stock"

	if err != nil {
		slog.Error("Error adding the watchlist:", slog.Any("err", err))
		response = "error adding the watched stock"
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func listHandler(event *events.ApplicationCommandInteractionCreate) {
	watches, err := database.GetUserWatchList(event.User().ID.String())

	if err != nil {
		slog.Error("Error fetching watchlists:", slog.Any("err", err))
	}

	var embed discord.Embed

	embed.Title = "Watched Stocks"

	if len(watches) == 0 {
		embed.Description = "No watched stock yet"

		_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed},
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", embed))
		}
		return
	}

	for _, watch := range watches {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:  watch.Symbol,
			Value: fmt.Sprintf("%.2f", watch.PriceTarget),
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
	err := database.RemoveWatchList(event.User().ID.String(), strings.ToUpper(args.Options["symbol"].String()))
	response := "Successfully removed the watched stock"

	if err != nil {
		slog.Error("Error deleting the watchlist:", slog.Any("err", err))
		response = "error removing the watched stock"
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Content: &response,
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err))
	}
}

func (s WatchCommand) CreateCommandArguments() []discord.ApplicationCommandOption {
	return []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "add",
			Description: "watch a stock",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "symbol",
					Description: "stock symbol",
					Required:    true,
				},
				discord.ApplicationCommandOptionFloat{
					Name:        "price",
					Description: "price target",
					Required:    true,
				},
				discord.ApplicationCommandOptionBool{
					Name:        "above",
					Description: "if the price needs to be above the target",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "list",
			Description: "list watched stock",
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "update",
			Description: "update a watched stock",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "symbol",
					Description: "stock symbol",
					Required:    true,
				},
				discord.ApplicationCommandOptionFloat{
					Name:        "price",
					Description: "price target",
					Required:    true,
				},
				discord.ApplicationCommandOptionBool{
					Name:        "above",
					Description: "if the price needs to be above the target",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "remove",
			Description: "remove a watched stock",
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
