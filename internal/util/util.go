package util

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

const (
	DISCORD_EMOJI_URL       = "https://cdn.discordapp.com/emojis/%s.%s"
	DiscordEpoch      int64 = 1420070400000
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
	}

	if start.Weekday() == time.Saturday {
		start = start.AddDate(0, 0, -1)
	} else if start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, -2)
	}

	return ticker.History(yfa.HistoryQuery{
		Start:    start.Format("2006-01-02"),
		End:      fmt.Sprintf("%d", end.Unix()),
		Interval: "1d",
	})
}
