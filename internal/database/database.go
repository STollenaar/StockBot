package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2" // DuckDB Go driver
	"github.com/stollenaar/stockbot/internal/util"
	"github.com/stollenaar/stockbot/internal/util/yfa"
)

var (
	duckdbClient *sql.DB

	//go:embed changelog/*.sql
	changeLogFiles embed.FS
)

func Exit() {
	duckdbClient.Close()
}

func init() {

	var err error

	duckdbClient, err = sql.Open("duckdb", fmt.Sprintf("%s/stockbot.db", util.ConfigFile.DUCKDB_PATH))

	if err != nil {
		log.Fatal(err)
	}

	// Ensure changelog table exists
	_, err = duckdbClient.Exec(`
	CREATE TABLE IF NOT EXISTS database_changelog (
		id INTEGER PRIMARY KEY,
		name VARCHAR NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		checksum VARCHAR,
		success BOOLEAN DEFAULT TRUE
	);
	`)

	if err != nil {
		log.Fatalf("failed to create changelog table: %v", err)
	}

	if err := runMigrations(); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	slog.Info("All migrations applied successfully.")
}

func runMigrations() error {
	entries, err := changeLogFiles.ReadDir("changelog")
	if err != nil {
		return fmt.Errorf("failed to read embedded changelogs: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)

	for i, file := range files {
		id := i + 1

		contents, err := changeLogFiles.ReadFile(filepath.Join("changelog", file))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		checksum := sha256.Sum256(contents)
		checksumHex := hex.EncodeToString(checksum[:])

		var appliedChecksum string
		err = duckdbClient.QueryRow("SELECT checksum FROM database_changelog WHERE id = ?", id).Scan(&appliedChecksum)
		if err == nil {
			if appliedChecksum != checksumHex {
				return fmt.Errorf("checksum mismatch for migration %s (id=%d). File has changed", file, id)
			}
			log.Printf("Skipping already applied migration %s", file)
			continue
		}

		// Run changelogs in a transaction
		tx, err := duckdbClient.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin tx: %w", err)
		}

		_, err = tx.Exec(string(contents))
		if err != nil {
			_ = tx.Rollback()
			_, _ = duckdbClient.Exec(`
				INSERT INTO database_changelog (id, name, applied_at, checksum, success) VALUES (?, ?, ?, ?, false)
			`, id, file, time.Now(), checksumHex)
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		_, err = duckdbClient.Exec(`
			INSERT INTO database_changelog (id, name, applied_at, checksum, success)
			VALUES (?, ?, ?, ?, true)
		`, id, file, time.Now(), checksumHex)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		log.Printf("Applied migration %s", file)
	}

	return nil
}

type Portfolio struct {
	UserID string
	Symbol string
	Shares float64
}

func (p Portfolio) Values() []interface{} {
	return []interface{}{p.UserID, p.Symbol, p.Shares}
}

type WatchList struct {
	UserID      string
	Symbol      string
	PriceTarget float64
	Triggered   bool
	Direction   bool
}

func (w WatchList) Values() []interface{} {
	return []interface{}{w.UserID, w.Symbol, w.PriceTarget, w.Direction}
}

type StockPrice struct {
	Symbol string
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

func (s StockPrice) Values() []interface{} {
	return []interface{}{s.Symbol, s.Date, s.Open, s.High, s.Low, s.Close, s.Volume}
}

func GetCompletePortfolio(userID string) (portfolio []Portfolio, err error) {
	rows, err := duckdbClient.Query(`SELECT * FROM portfolios WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var port Portfolio

		err = rows.Scan(&port.UserID, &port.Symbol, &port.Shares)
		if err != nil {
			break
		}
		portfolio = append(portfolio, port)
	}
	return
}

func GetPortfolio(userID, symbol string) (portfolio Portfolio, err error) {
	rows := duckdbClient.QueryRow(`SELECT * FROM portfolios WHERE user_id = ? AND symbol = ?;`, userID, symbol)

	var port Portfolio

	err = rows.Scan(&port.UserID, &port.Symbol, &port.Shares)

	return port, err
}

func RemovePortfolio(userID, symbol string) error {
	_, err := duckdbClient.Exec("DELETE FROM portfolios WHERE user_id = ? AND symbol = ?;", userID, symbol)
	return err
}

func (p *Portfolio) UpsertPortfolio() error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	AddTrackedStock(p.Symbol)

	_, err = tx.Exec(`
		INSERT INTO portfolios (user_id, symbol, shares)
		VALUES (?, ?, ?) 
		ON CONFLICT DO UPDATE SET 
		shares = EXCLUDED.shares;
	`, p.UserID, p.Symbol, p.Shares)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func GetUserWatchList(userID string) (watchlists []WatchList, err error) {
	rows, err := duckdbClient.Query(`SELECT * FROM watchlists WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var watchlist WatchList

		err = rows.Scan(&watchlist.UserID, &watchlist.Symbol, &watchlist.PriceTarget, &watchlist.Direction, &watchlist.Triggered)
		if err != nil {
			break
		}
		watchlists = append(watchlists, watchlist)
	}
	return
}

func GetWatchLists() (watchlists []WatchList, err error) {
	rows, err := duckdbClient.Query(`SELECT * FROM watchlists WHERE triggered = false;`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var watchlist WatchList

		err = rows.Scan(&watchlist.UserID, &watchlist.Symbol, &watchlist.PriceTarget, &watchlist.Direction, &watchlist.Triggered)
		if err != nil {
			break
		}
		watchlists = append(watchlists, watchlist)
	}
	return
}

func RemoveWatchList(userID, symbol string) error {
	_, err := duckdbClient.Exec("DELETE FROM watchlists WHERE user_id = ? AND symbol = ?;", userID, symbol)
	return err
}

func (w *WatchList) UpsertWatchlist() error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	AddTrackedStock(w.Symbol)

	_, err = tx.Exec(`
		INSERT INTO watchlists (user_id, symbol, price_target, direction)
		VALUES (?, ?, ?, ?) 
		ON CONFLICT DO UPDATE SET 
		price_target = EXCLUDED.price_target,
		direction = EXCLUDED.direction;
	`, w.UserID, w.Symbol, w.PriceTarget, w.Direction)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (w *WatchList) SetTriggerWatchlist() error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE watchlists 
		SET triggered = true
		WHERE user_id = ? AND symbol = ?;
		`, w.UserID, w.Symbol)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// IsTrackedStock checks whether the given symbol exists in tracked_stocks.
func IsTrackedStock(symbol string) (bool, error) {
	var existsInt int
	row := duckdbClient.QueryRow(`SELECT EXISTS(SELECT 1 FROM tracked_stocks WHERE symbol = ?);`, symbol)
	if err := row.Scan(&existsInt); err != nil {
		return false, err
	}
	return existsInt == 1, nil
}

func GetTrackedStocks() (tracked []string, err error) {
	rows, err := duckdbClient.Query(`SELECT * FROM tracked_stocks;`)

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var symbol string
		err := rows.Scan(&symbol)
		if err != nil {
			continue
		}
		tracked = append(tracked, symbol)
	}
	return
}

// AddTrackedStock inserts the symbol into tracked_stocks (no-op if already present).
func AddTrackedStock(symbol string) error {
	if ok, _ := IsTrackedStock(symbol); !ok {
		_, err := duckdbClient.Exec(`INSERT INTO tracked_stocks (symbol) VALUES (?) ON CONFLICT DO NOTHING;`, symbol)

		if err != nil {
			return err
		}

		ticker := yfa.NewTicker(symbol)
		hist, err := yfa.FetchHistory(ticker)

		if err != nil {
			slog.Error("failed getting 5year history", slog.Any("err", err))
			return err
		}

		var stockPrices []StockPrice

		for date, price := range hist {
			// parse the date key into a time.Time (stored as UTC)
			parsedDate, _ := time.ParseInLocation("2006-01-02", date, time.UTC)

			// build StockPrice and persist
			stockPrices = append(stockPrices, StockPrice{
				Symbol: symbol,
				Date:   parsedDate,
				Open:   price.Open,
				High:   price.High,
				Low:    price.Low,
				Close:  price.Close,
				Volume: price.Volume,
			})
		}

		if err := SetStockPrices(stockPrices); err != nil {
			slog.Error("failed to set stock price", slog.Any("err", err), slog.String("symbol", symbol))
		}
	}
	return nil
}

// RemoveTrackedStock deletes the symbol from tracked_stocks and optionally its prices.
func RemoveTrackedStock(symbol string) error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM stock_prices WHERE symbol = ?;`, symbol)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM tracked_stocks WHERE symbol = ?;`, symbol)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetStockPrices returns stock prices for a symbol between start and end (inclusive),
// ordered by date ascending.
func GetStockPrices(symbol string, start, end time.Time) (prices []StockPrice, err error) {
	rows, err := duckdbClient.Query(`
        SELECT symbol, date, open, high, low, close, volume
        FROM stock_prices
        WHERE symbol = ? AND date >= ? AND date <= ?
        ORDER BY date ASC;
    `, symbol, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sp StockPrice
		if err := rows.Scan(&sp.Symbol, &sp.Date, &sp.Open, &sp.High, &sp.Low, &sp.Close, &sp.Volume); err != nil {
			slog.Error("failed scanning stock price row", slog.Any("err", err))
			return nil, err
		}
		prices = append(prices, sp)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return prices, nil
}

// SetStockPrice inserts or updates a price row for the given symbol/date.
// volume can be 0 if unknown.
func SetStockPrice(stock StockPrice) error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        INSERT INTO stock_prices (symbol, date, open, high, low, close, volume)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT DO UPDATE SET
            open = EXCLUDED.open,
            high = EXCLUDED.high,
            low = EXCLUDED.low,
            close = EXCLUDED.close,
            volume = EXCLUDED.volume;
    `, stock.Symbol, stock.Date, stock.Open, stock.High, stock.Low, stock.Close, stock.Volume)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// SetStockPrices inserts or updates a price row for the given symbol/date.
// volume can be 0 if unknown.
func SetStockPrices(stocks []StockPrice) error {
	tx, err := duckdbClient.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stock := range stocks {
		_, err = tx.Exec(`
        INSERT INTO stock_prices (symbol, date, open, high, low, close, volume)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT DO UPDATE SET
            open = EXCLUDED.open,
            high = EXCLUDED.high,
            low = EXCLUDED.low,
            close = EXCLUDED.close,
            volume = EXCLUDED.volume;
    `, stock.Symbol, stock.Date, stock.Open, stock.High, stock.Low, stock.Close, stock.Volume)
		if err != nil {
			slog.Error("failed committing stock price", slog.Any("err", err))
		}
	}

	return tx.Commit()
}

// RemoveStockPrice deletes a single price row for the given symbol and date.
func RemoveStockPrice(symbol string, date time.Time) error {
	_, err := duckdbClient.Exec(`DELETE FROM stock_prices WHERE symbol = ? AND date = ?;`, symbol, date)
	return err
}
