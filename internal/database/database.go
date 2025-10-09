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

type WatchList struct {
	UserID      string
	Symbol      string
	PriceTarget float64
	Triggered   bool
	Direction   bool
}

func GetPortfolio(userID string) (portfolio []Portfolio, err error) {
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
