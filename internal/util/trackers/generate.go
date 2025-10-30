package trackers

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

var (
	WORKING_DIR = os.Getenv("PWD")
)

func PeriodChange(period string, hist map[string]yfa.PriceData) string {
	var start, end time.Time
	end = time.Now()

	switch period {
	case "1wk":
		start = end.AddDate(0, 0, -7)
	case "1mo":
		start = end.AddDate(0, -1, -1)
	case "3mo":
		start = end.AddDate(0, -3, -1)
	case "1y":
		start = end.AddDate(-1, 0, 0)
	case "5y":
		start = end.AddDate(-5, 0, 0)
	default:
		return ""
	}

	if start.Weekday() == time.Saturday {
		start = start.AddDate(0, 0, -1)
	} else if start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, -2)
	}

	if end.Weekday() == time.Saturday {
		end = end.AddDate(0, 0, -1)
	} else if end.Weekday() == time.Sunday {
		end = end.AddDate(0, 0, -2)
	}

	if _, ok := hist[start.Format("2006-01-02")]; !ok {
		return "N/A"
	}

	if _, ok := hist[end.Format("2006-01-02")]; !ok {
		return "N/A"
	}

	if len(hist) < 2 {
		return "N/A"
	}

	startPrice := hist[start.Format("2006-01-02")].Close
	endPrice := hist[end.Format("2006-01-02")].Close
	percentChange := ((endPrice - startPrice) / startPrice) * 100

	return fmt.Sprintf("%.2f%%", percentChange)
}

func FetchHistory(ticker *yfa.Ticker, period string) (yfa.PriceHistory, error) {
	var start, end time.Time
	end = time.Now()
	interval := "1d"

	switch period {
	case "1d":
		start = end.AddDate(0, 0, -1)
		interval = "1m"
	case "1wk":
		start = end.AddDate(0, 0, -7)
	case "1mo":
		start = end.AddDate(0, -1, -1)
	case "3mo":
		start = end.AddDate(0, -3, -1)
	case "1y":
		start = end.AddDate(-1, 0, 0)
	case "5y":
		start = end.AddDate(-5, 0, 0)
	default:
		return yfa.PriceHistory{}, fmt.Errorf("unsupported period: %s", period)
	}

	// shift start/end off weekend
	if start.Weekday() == time.Saturday {
		start = start.AddDate(0, 0, -1)
	} else if start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, -2)
	}

	if end.Weekday() == time.Saturday {
		end = end.AddDate(0, 0, -1)
	} else if end.Weekday() == time.Sunday {
		end = end.AddDate(0, 0, -2)
	}

	var daily, yearly map[string]yfa.PriceData

	yearStart := end.AddDate(-1, 0, 0)

	rows, dbErr := database.GetStockPrices(ticker.Symbol, yearStart, end)
	if dbErr == nil && len(rows) > 0 {
		yearly = stockPriceToPriceData(rows)
	}

	// if DB miss, attempt to fetch yearly from yahoo
	if yearly == nil {
		if hist, hErr := ticker.History(yfa.HistoryQuery{
			Start:    yearStart.Format("2006-01-02"),
			End:      fmt.Sprintf("%d", end.Unix()),
			Interval: "1d",
		}); hErr == nil {
			yearly = hist
		}
	}

	if period == "1d" {
		hist, err := ticker.History(yfa.HistoryQuery{
			Start:    start.Format("2006-01-02"),
			End:      fmt.Sprintf("%d", end.Unix()),
			Interval: interval, // "1m"
		})
		if err != nil {
			return yfa.PriceHistory{}, err
		}
		daily = hist
	} else {
		daily = nil
	}

	return yfa.PriceHistory{
		Daily:  daily,
		Yearly: yearly,
	}, nil
}

func stockPriceToPriceData(rows []database.StockPrice) map[string]yfa.PriceData {
	m := make(map[string]yfa.PriceData, len(rows))
	for _, r := range rows {
		key := r.Date.UTC().Format("2006-01-02")
		m[key] = yfa.PriceData{
			Open:   r.Open,
			High:   r.High,
			Low:    r.Low,
			Close:  r.Close,
			Volume: r.Volume,
		}
	}
	return m
}

func GenerateLineChart(hist map[string]yfa.PriceData, info yfa.YahooTickerInfo, period string) *discord.File {
	t := time.Now()
	tmp, err := os.CreateTemp(WORKING_DIR, fmt.Sprintf("chart-%d-*.png", t.UnixNano()))
	if err != nil {
		slog.Error("Error creating temp file", slog.Any("err", err))
		return nil
	}
	tmpName := tmp.Name()
	tmp.Close()

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "#FFFFFF",
			Width:           "100%",
		}),

		charts.WithAnimation(false),
		charts.WithTitleOpts(opts.Title{
			Title: fmt.Sprintf("%s over %s", info.Symbol, periodToFriendlyName(period)),
			Right: "40%",
		}),
		charts.WithYAxisOpts(
			opts.YAxis{
				Name:         fmt.Sprintf("Price (%s)", info.Currency),
				Position:     "left",
				NameLocation: "middle",
				NameGap:      25,
			},
		),
		charts.WithXAxisOpts(
			opts.XAxis{
				Name:         "Date",
				Position:     "bottom",
				NameLocation: "center",
				NameGap:      25,
			},
		),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)
	keys := slices.Collect(maps.Keys(hist))
	slices.Sort(keys)

	var axes []string
	var values []yfa.PriceData
	for _, k := range keys {
		if hist[k].Close == 0 {
			continue
		}

		values = append(values, hist[k])
		if period == "1d" {
			k = strings.Split(k, " ")[1]
		}
		axes = append(axes, k)
	}

	line.SetXAxis(axes).
		AddSeries("Date", genLineData(info.Symbol, values)).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				ShowSymbol: opts.Bool(false),
			}),
			charts.WithLabelOpts(opts.Label{
				Show: opts.Bool(true),
			}),
		)

	err = render.MakeChartSnapshot(line.RenderContent(), tmpName)
	if err != nil {
		slog.Error("Error rendering image", slog.Any("err", err))
		os.Remove(tmpName)
		return nil
	}

	image, err := os.ReadFile(tmpName)
	os.Remove(tmpName)
	if err != nil {
		slog.Error("Error reading temp image", slog.Any("err", err))
		return nil
	}

	imgReader := bytes.NewReader(image)

	return &discord.File{
		Name:   filepath.Base(tmpName),
		Reader: imgReader,
	}
}

func genLineData(symbol string, values []yfa.PriceData) (rs []opts.LineData) {
	rs = make([]opts.LineData, 0, len(values))
	for _, data := range values {
		rs = append(rs, opts.LineData{Name: symbol, Value: data.Close})
	}
	return
}

func periodToFriendlyName(period string) string {
	switch period {
	case "1d":
		return "1 Day"
	case "1wk":
		return "1 Week"
	case "1mo":
		return "1 Month"
	case "3mo":
		return "3 Month"
	case "1y":
		return "1 Year"
	case "5y":
		return "5 Year"
	default:
		return ""
	}
}
