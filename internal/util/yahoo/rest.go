package yahoo

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
)

const (
	BASE_URL = "https://query2.finance.yahoo.com/v1"
)

func Request(symbol string) map[string]interface{} {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s")

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("Error fetchin symbol", slog.Any("err", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatal(err)
	}
	return data
}
