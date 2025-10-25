package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

const (
	DISCORD_EMOJI_URL       = "https://cdn.discordapp.com/emojis/%s.%s"
	DiscordEpoch      int64 = 1420070400000
)

var (
	WORKING_DIR = os.Getenv("PWD")
)

// Contains check slice contains want string
func Contains(slice []string, want string) bool {
	for _, element := range slice {
		if element == want {
			return true
		}
	}
	return false
}

// DeleteEmpty deleting empty strings in string slice
func DeleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

// Elapsed timing time till function completion
func Elapsed(channel string) func() {
	start := time.Now()
	return func() {
		fmt.Printf("Loading %s took %v to complete\n", channel, time.Since(start))
	}
}

// FilterDiscordMessages filtering specific messages out of message slice
func FilterDiscordMessages(messages []*discord.Message, condition func(*discord.Message) bool) (result []*discord.Message) {
	for _, message := range messages {
		if condition(message) {
			result = append(result, message)
		}
	}
	return result
}

// SnowflakeToTimestamp converts a Discord snowflake ID to a timestamp
func SnowflakeToTimestamp(snowflakeID string) (time.Time, error) {
	id, err := strconv.ParseInt(snowflakeID, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	timestamp := (id >> 22) + DiscordEpoch
	return time.Unix(0, timestamp*int64(time.Millisecond)), nil
}

// FetchDiscordEmojiImage fetches the raw image bytes for a given emoji ID and animation status.
func FetchDiscordEmojiImage(emojiID string, isAnimated bool) (string, error) {
	ext := "png"
	if isAnimated {
		ext = "gif"
	}
	url := fmt.Sprintf(DISCORD_EMOJI_URL, emojiID, ext)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch emoji from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}
	base64Data := base64.StdEncoding.EncodeToString(data)

	return base64Data, nil
}

func GetSeparator() discord.SeparatorComponent {
	divider := true

	return discord.SeparatorComponent{
		Divider: &divider,
		Spacing: discord.SeparatorSpacingSizeLarge,
	}
}

func Pointer[T any](d T) *T {
	return &d
}

func PeriodChangeEmbed(period string, hist map[string]yfa.PriceData) discord.EmbedField {
	var label string
	var start, end time.Time
	end = time.Now()

	switch period {
	case "1wk":
		start = end.AddDate(0, 0, -7)
		label = "Weekly % Change"
	case "1mo":
		start = end.AddDate(0, -1, -1)
		label = "Monthly % Change"
	case "3mo":
		start = end.AddDate(0, -3, -1)
		label = "Monthly % Change"
	case "1y":
		start = end.AddDate(-1, 0, 0)
		label = "Yearly % Change"
	case "5y":
		start = end.AddDate(-5, 0, 0)
		label = "Yearly % Change"
	default:
		return discord.EmbedField{}
	}

	if start.Weekday() == time.Saturday {
		start = start.AddDate(0, 0, -1)
	} else if start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, -2)
	}

	if _, ok := hist[start.Format("2006-01-02")]; !ok {
		return discord.EmbedField{
			Name:  label,
			Value: "N/A",
		}
	}

	if _, ok := hist[end.Format("2006-01-02")]; !ok {
		return discord.EmbedField{
			Name:  label,
			Value: "N/A",
		}
	}

	if len(hist) < 2 {
		return discord.EmbedField{
			Name:  label,
			Value: "N/A",
		}
	}

	startPrice := hist[start.Format("2006-01-02")].Close
	endPrice := hist[end.Format("2006-01-02")].Close
	percentChange := ((endPrice - startPrice) / startPrice) * 100

	return discord.EmbedField{
		Name:   label,
		Value:  fmt.Sprintf("%.2f%%", percentChange),
		Inline: Pointer(true),
	}
}

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

func FetchHistory(ticker *yfa.Ticker, period string) (map[string]yfa.PriceData, error) {
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

	return ticker.History(yfa.HistoryQuery{
		Start:    start.Format("2006-01-02"),
		End:      fmt.Sprintf("%d", end.Unix()),
		Interval: interval,
	})
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
		// Don't forget disable the Animation
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

type Button struct {
	ID     string
	Label  string
	Active bool
}

func GenerateButtons(buttons []Button) (components []discord.InteractiveComponent) {
	for _, button := range buttons {
		if button.Active {
			components = append(components,
				discord.ButtonComponent{
					CustomID: button.ID,
					Label:    button.Label,
					Style:    discord.ButtonStylePrimary,
				},
			)
		} else {
			components = append(components,
				discord.ButtonComponent{
					CustomID: button.ID,
					Label:    button.Label,
					Style:    discord.ButtonStyleSecondary,
				},
			)
		}

	}
	return
}
