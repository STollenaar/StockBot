package util

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/disgoorg/disgo/discord"
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
