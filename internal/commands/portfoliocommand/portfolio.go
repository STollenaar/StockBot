package portfoliocommand

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util"
	"github.com/stollenaar/stockbot/internal/util/yfa"
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
	components, files := generateComponents(portfolios)

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

func generateComponents(portfolio []database.Portfolio) (components []discord.LayoutComponent, files []*discord.File) {
	for _, item := range portfolio {

		ticker := yfa.NewTicker(item.Symbol)
		// get the latest PriceData
		info, err := ticker.Info()

		if err != nil {
			slog.Error("Error fetching stock", slog.Any("err", err))
			continue
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
			slog.Error("Error fetching history", slog.Any("err", err))
			continue
		}

		chart := generateLineChart(hist, item.Symbol)
		files = append(files, chart)
		shares := fmt.Sprintf("%.2f", item.Shares)

		components = append(components,
			discord.ContainerComponent{
				Components: []discord.ContainerSubComponent{
					discord.TextDisplayComponent{
						Content: fmt.Sprintf("# %s", item.Symbol),
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
								URL: fmt.Sprintf("attachment://%s", chart.Name),
							},
						},
					},
				},
			},
		)
	}
	return
}

func generateLineChart(hist map[string]yfa.PriceData, symbol string) *discord.File {
	t := time.Now()
	fileName := fmt.Sprintf("%d.png", t.UnixNano())
	var image []byte

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),
		// Don't forget disable the Animation
		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: symbol,
			Right: "40%",
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)
	axes := slices.Collect(maps.Keys(hist))
	slices.Sort(axes)

	var values []yfa.PriceData
	for _, k := range axes {
		values = append(values, hist[k])
	}

	line.SetXAxis(axes).
		AddSeries("Date", genLineData(values)).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				ShowSymbol: opts.Bool(false),
			}),
			charts.WithLabelOpts(opts.Label{
				Show: opts.Bool(true),
			}),
		)

	err := render.MakeChartSnapshot(line.RenderContent(), fileName)
	if err != nil {
		slog.Error("Error rendering image", slog.Any("err", err))
		return nil
	}

	image, _ = os.ReadFile(fileName)
	os.Remove(fileName)
	imgReader := bytes.NewReader(image)

	return &discord.File{
		Name:   fileName,
		Reader: imgReader,
	}
}

func genLineData(values []yfa.PriceData) (rs []opts.LineData) {
	for _, data := range values {
		rs = append(rs, opts.LineData{Value: data.Close})
	}
	return
}
