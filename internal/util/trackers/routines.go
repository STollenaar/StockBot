package trackers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/stollenaar/stockbot/internal/database"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

func StartChecker(client *bot.Client) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			now := time.Now()
			if now.Weekday() != time.Saturday && now.Weekday() != time.Sunday {
				CheckAlerts(client)
			}
		}
	}()

	scheduleDailyRefresh()
}

// scheduleDailyRefresh triggers once per day at 23:00 UTC.
func scheduleDailyRefresh() {
	go func() {
		for {
			now := time.Now().UTC()
			target := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, time.UTC)
			if !target.After(now) {
				target = target.Add(24 * time.Hour)
			}
			sleep := time.Until(target)
			time.Sleep(sleep)

			RefreshTrackedStocks()
		}
	}()
}

func RefreshTrackedStocks() {
	trackedStock, err := database.GetTrackedStocks()
	if err != nil {
		slog.Error("Error fetching tracked stocks:", slog.Any("err", err))
		return
	}

	for _, symbol := range trackedStock {
		ticker := yfa.NewTicker(symbol)

		// fetch history for the current UTC day
		now := time.Now().UTC()
		day := now.Format("2006-01-02")
		hist, err := ticker.History(yfa.HistoryQuery{
			Start:    day,
			End:      day,
			Interval: "1d",
		})

		if err != nil {
			slog.Error("Error fetching history:", slog.Any("err", err), slog.String("symbol", symbol))
			continue
		}

		var pd yfa.PriceData
		var dateKey string
		if v, ok := hist[day]; ok {
			pd = v
			dateKey = day
		} else {
			// fallback to the most recent available key in the returned map
			for k := range hist {
				if dateKey == "" || k > dateKey {
					dateKey = k
				}
			}
			if dateKey == "" {
				// no data available
				slog.Debug("no history rows returned", slog.String("symbol", symbol))
				continue
			}
			pd = hist[dateKey]
		}

		// parse the date key into a time.Time (stored as UTC)
		parsedDate, err := time.ParseInLocation("2006-01-02", dateKey, time.UTC)
		if err != nil {
			// fallback to now UTC if parsing fails
			parsedDate = now
		}

		// build StockPrice and persist
		sp := database.StockPrice{
			Symbol: symbol,
			Date:   parsedDate,
			Open:   pd.Open,
			High:   pd.High,
			Low:    pd.Low,
			Close:  pd.Close,
			Volume: int64(pd.Volume),
		}

		if err := database.SetStockPrice(sp); err != nil {
			slog.Error("failed to set stock price", slog.Any("err", err), slog.String("symbol", symbol), slog.Time("date", parsedDate))
			continue
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func CheckAlerts(client *bot.Client) {
	watchlists, err := database.GetWatchLists()

	if err != nil {
		slog.Error("Error fetching watchlists:", slog.Any("err", err))
		return
	}

	grouped := make(map[string][]database.WatchList)
	for _, watched := range watchlists {
		grouped[watched.Symbol] = append(grouped[watched.Symbol], watched)
	}

	for symbol, lists := range grouped {
		ticker := yfa.NewTicker(symbol)
		// get the latest PriceData
		info, err := ticker.Info()

		if err != nil {
			continue
		}

		toMention := make(map[bool][]string)
		for _, w := range lists {
			if w.Direction && info.RegularMarketPrice.Raw >= w.PriceTarget {
				toMention[true] = append(toMention[true], w.UserID)
				flk, err := snowflake.Parse(w.UserID)
				if err != nil {
					continue
				}
				dmChannel, _ := client.Rest.CreateDMChannel(flk)
				client.Rest.CreateMessage(dmChannel.ID(), discord.MessageCreate{
					Content: fmt.Sprintf("This is a price alert for %s\nThe current price is %s which is above your target of %.2f", w.Symbol, info.RegularMarketPrice.Fmt, w.PriceTarget),
				})
				w.SetTriggerWatchlist()
			} else if !w.Direction && info.RegularMarketPrice.Raw <= w.PriceTarget {
				toMention[false] = append(toMention[false], w.UserID)
				flk, err := snowflake.Parse(w.UserID)
				if err != nil {
					continue
				}
				dmChannel, _ := client.Rest.CreateDMChannel(flk)
				client.Rest.CreateMessage(dmChannel.ID(), discord.MessageCreate{
					Content: fmt.Sprintf("This is a price alert for %s\nThe current price is %s which is below your target of %.2f", w.Symbol, info.RegularMarketPrice.Fmt, w.PriceTarget),
				})
				w.SetTriggerWatchlist()
			}
		}
	}
}
