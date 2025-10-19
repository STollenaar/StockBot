package portfoliocommand

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

var (
	PERIODS = []string{"5y", "1y", "3mo", "1mo", "1wk"}

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
	portfolios, err := database.GetCompletePortfolio(event.User().ID.String())

	if err != nil {
		slog.Error("Error fetching portfolio:", slog.Any("err", err))
	}

	var components []discord.LayoutComponent

	if len(portfolios) == 0 {
		components = append(components,
			discord.TextDisplayComponent{
				Content: "No stock in your portfolio yet",
			})

		_, err := event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
			Components: &components,
			Flags:      util.ConfigFile.SetComponentV2Flags(),
		})
		if err != nil {
			slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", components))
		}
		return
	}
	components, files := generateComponents("1y", portfolios)

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Components: &components,
		Files:      files,
		Flags:      util.ConfigFile.SetComponentV2Flags(),
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", components))
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

func (s PortfolioCommand) ComponentHandler(event *events.ComponentInteractionCreate) {
	if event.Message.Interaction.User.ID != event.Member().User.ID {
		return
	}

	err := event.DeferUpdateMessage()

	if err != nil {
		slog.Error("Error deferring: ", slog.Any("err", err))
		return
	}

	components := event.Message.Components
	details := strings.Split(event.Data.CustomID(), ";")

	pIndex, _ := strconv.Atoi(details[1])
	portfolio, err := database.GetPortfolio(event.Member().User.ID.String(), details[2])

	if err != nil {
		slog.Error("Error fetching portfolio: ", slog.Any("err", err))
		return
	}

	component, file := generateComponent(pIndex, details[3], portfolio)
	components[pIndex] = component

	var attachments []discord.AttachmentUpdate
	for i, attachment := range components {
		if i != pIndex {
			id := attachment.(discord.ContainerComponent).Components[1].(discord.SectionComponent).Accessory.(discord.ThumbnailComponent).Media.AttachmentID
			if id == snowflake.MustParse("0"){
				continue
			}
			attachments = append(attachments,
				discord.AttachmentKeep{
					ID: id,
				},
			)
		}
	}

	_, err = event.Client().Rest.UpdateInteractionResponse(event.ApplicationID(), event.Token(), discord.MessageUpdate{
		Components:  &components,
		Attachments: &attachments,
		Files:       []*discord.File{file},
		Flags:       util.ConfigFile.SetComponentV2Flags(),
	})
	if err != nil {
		slog.Error("Error editing the response:", slog.Any("err", err), slog.Any(". With body:", components))
	}

}

func generateComponents(period string, portfolio []database.Portfolio) (components []discord.LayoutComponent, files []*discord.File) {
    // bounded concurrency
    const maxConcurrent = 4
    sem := make(chan struct{}, maxConcurrent)
    type res struct {
        idx       int
        component discord.LayoutComponent
        file      *discord.File
    }
    out := make(chan res, len(portfolio))

    for i, p := range portfolio {
        sem <- struct{}{}
        go func(idx int, item database.Portfolio) {
            defer func() { <-sem }()
            component, file := generateComponent(idx, period, item)
            out <- res{idx: idx, component: component, file: file}
        }(i, p)
    }

    // collect results in order
    results := make([]res, len(portfolio))
    for i := 0; i < len(portfolio); i++ {
        r := <-out
        results[r.idx] = r
    }
    close(out)

    components = make([]discord.LayoutComponent, 0, len(portfolio))
    files = make([]*discord.File, 0, len(portfolio))
    for _, r := range results {
		if r.component == nil {
			continue
		}
        components = append(components, r.component)
        files = append(files, r.file)
    }
    return
}

func generateComponent(pIndex int, period string, portfolio database.Portfolio) (component discord.LayoutComponent, file *discord.File) {

	ticker := yfa.NewTicker(portfolio.Symbol)
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
	shares := fmt.Sprintf("%.2f", portfolio.Shares)

	var prevPeriod, nextPeriod string

	index := slices.Index(PERIODS, period)

	if index != -1 {
		if index > 0 {
			prevPeriod = PERIODS[index-1]
		}
		if index < len(PERIODS)-1 {
			nextPeriod = PERIODS[index+1]
		}
	}

	component = discord.ContainerComponent{
		ID: pIndex,
		Components: []discord.ContainerSubComponent{
			discord.TextDisplayComponent{
				Content: fmt.Sprintf("# %s", portfolio.Symbol),
			},
			discord.SectionComponent{
				Components: []discord.SectionSubComponent{
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Amount of Shares:** %s\n~~%s~~", shares, strings.Repeat(" ", 18+len(shares))),
					},
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("**Daily %% Change:** %s\n**Weekly %% Change:** %s\n**Yearly %% Change:** %s", info.RegularMarketChangePercent.Fmt, util.PeriodChange("1wk", hist), util.PeriodChange("1y", hist)),
					},
				},
				Accessory: discord.ThumbnailComponent{
					Media: discord.UnfurledMediaItem{
						URL: fmt.Sprintf("attachment://%s", file.Name),
					},
				},
			},
			discord.ActionRowComponent{
				Components: []discord.InteractiveComponent{
					discord.ButtonComponent{
						CustomID: fmt.Sprintf("portfolio;%d;%s;%s;-", pIndex, info.Symbol, prevPeriod),
						Style:    discord.ButtonStylePrimary,
						Label:    "-",
					},
					discord.ButtonComponent{
						CustomID: fmt.Sprintf("portfolio;%d;%s;%s;+", pIndex, info.Symbol, nextPeriod),
						Style:    discord.ButtonStylePrimary,
						Label:    "+",
					},
				},
			},
		},
	}
	return
}
