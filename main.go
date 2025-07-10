package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

// Category represents a CoinMarketCap category
type Category struct {
	Id              string  `json:"id"`
	Name            string  `json:"name"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	NumTokens       int     `json:"num_tokens"`
	AvgPriceChange  float64 `json:"avg_price_change"`
	MarketCap       float64 `json:"market_cap"`
	MarketCapChange float64 `json:"market_cap_change"`
	Volume          float64 `json:"volume"`
	VolumeChange    float64 `json:"volume_change"`
}

// CategoryResponse holds the response for the categories endpoint
type CategoryResponse struct {
	Data []Category `json:"data"`
}

// Coin represents a cryptocurrency in a category
type Coin struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Quote  struct {
		USD struct {
			Volume24h float64 `json:"volume_24h"`
			MarketCap float64 `json:"market_cap"`
		} `json:"USD"`
	} `json:"quote"`
}

// CategoryCoinsResponse holds the response for the category coins endpoint
type CategoryCoinsResponse struct {
	Data struct {
		Coins []Coin `json:"coins"`
	} `json:"data"`
}

// Config holds the API configuration
type Config struct {
	APIKey  string
	BaseURL string
}

var allowedCategories = map[string]string{
	"6433de7df79a2653906cd680": "Layer 1[120]",
	"67c514446feebc2b5bcc23f1": "US Strategic Crypto Reserve[5]",
	"604f2772ebccdd50cd175fd9": "Coinbase Ventures Portfolio[63]",
	"63feda8ad0a19758f3bde124": "Bitcoin Ecosystem[176]",
	"618c0beeb7dd913155b462f9": "Ethereum Ecosystem[3411]",
	"5fb62883c9ddcc213ed13308": "DeFi[1998]",
	"604f2753ebccdd50cd175fc1": "Stablecoin[226]",
	"6634dccba7b6f0637eec196a": "Fiat Stablecoin[26]",
	"60521ff1df5d3f36b84fbb61": "Solana Ecosystem[2212]",
	"60308028d2088f200c58a005": "BNB Chain Ecosystem[4029]",
	"6171122402ece807e8a9d3ed": "Arbitrum Ecosystem[556]",
	"60a5f6765abd81761fe58688": "Polygon Ecosystem[794]",
	"63c53f177e9034437b2a93bc": "Optimism Ecosystem[159]",
	"6051a82566fc1b42617d6dc6": "Memes[4473]",
	"6400b58c1701313dc2e853a9": "Real World Assets[159]",
	"604f2738ebccdd50cd175fac": "Decentralized Exchange (DEX) Token[194]",
	"6051a81a66fc1b42617d6db7": "AI & Big Data[779]",
	"6051a82166fc1b42617d6dc1": "Gaming[1017] (Gaming)",
	"6051a81b66fc1b42617d6db9": "Distributed Computing[130]",
	"604f2776ebccdd50cd175fdc": "Layer 2[56]",
	"63ff40541701313dc2e81ead": "Generative AI[91]",
	"6051a82366fc1b42617d6dc4": "IoT[63]",
}

func main() {
	binanceCoins := getBinanceCoins()

	// Load configuration
	config := Config{
		APIKey:  os.Getenv("CMC_API_KEY"), // Set your API key in environment variable CMC_API_KEY
		BaseURL: "https://pro-api.coinmarketcap.com",
	}

	if config.APIKey == "" {
		fmt.Println("Error: CMC_API_KEY environment variable is not set")
		os.Exit(1)
	}

	// Step 1: Get all categories
	categories, err := getCategories(config)
	if err != nil {
		fmt.Printf("Error fetching categories: %v\n", err)
		os.Exit(1)
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].MarketCap > categories[j].MarketCap
	})

	// Step 2: Get coins for each category
	for _, category := range categories {
		if category.NumTokens == 0 || strings.Contains(strings.ToLower(category.Name), "portfolio") {
			continue
		}

		if _, ok := allowedCategories[category.Id]; !ok {
			continue
		}

		fmt.Printf("\nCategory: %s[%d] (%s)\n", category.Name, category.NumTokens, category.Title)
		fmt.Println("ID:", category.Id)
		fmt.Println("Description:", category.Description)
		fmt.Printf("MarketCap B: %v (24h change: %v)\n", category.MarketCap/1_000_000_000, category.MarketCapChange)
		fmt.Printf("Vol B: %v (24h change: %v)\n", category.Volume/1_000_000_000, category.VolumeChange)
		fmt.Printf("Coins: [")
		coins, err := getCoinsForCategory(config, category.Id)
		if err != nil {
			log.Fatal(err)
		}
		coinNames := make([]string, 0, len(coins))
		for _, coin := range coins {
			if _, exists := binanceCoins[coin.Symbol]; !exists {
				continue
			}
			coinNames = append(coinNames, coin.Symbol)
		}
		fmt.Printf(strings.Join(coinNames, ", "))
		fmt.Println("]")
	}
}

// getCategories fetches the list of all categories from CoinMarketCap
func getCategories(config Config) ([]Category, error) {
	url := fmt.Sprintf("%s/v1/cryptocurrency/categories", config.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CMC_PRO_API_KEY", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response CategoryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil
}

// getCoinsForCategory fetches the list of coins for a specific category
func getCoinsForCategory(config Config, categoryID string) ([]Coin, error) {
	url := fmt.Sprintf("%s/v1/cryptocurrency/category?id=%s&limit=100&convert=USD", config.BaseURL, categoryID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CMC_PRO_API_KEY", config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response CategoryCoinsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data.Coins, nil
}

func getBinanceCoins() map[string]struct{} {
	spotURL := "https://api.binance.com/api/v3/exchangeInfo"
	usdmFuturesURL := "https://fapi.binance.com/fapi/v1/exchangeInfo"
	coinMFuturesURL := "https://dapi.binance.com/dapi/v1/exchangeInfo"

	// Initialize a map to store all unique assets
	allAssets := make(map[string]struct{})

	// Fetch and process spot market assets
	if symbols, err := getExchangeInfo(spotURL); err == nil {
		assets := extractAssets(symbols, true) // Include both baseAsset and quoteAsset
		for k := range assets {
			allAssets[k] = struct{}{}
		}
	} else {
		fmt.Println("Error fetching spot exchangeInfo:", err)
	}

	// Fetch and process USDM futures market assets
	if symbols, err := getExchangeInfo(usdmFuturesURL); err == nil {
		assets := extractAssets(symbols, true) // Include both baseAsset and quoteAsset
		for k := range assets {
			allAssets[k] = struct{}{}
		}
	} else {
		fmt.Println("Error fetching USDM futures exchangeInfo:", err)
	}

	// Fetch and process COIN-M futures market assets
	if symbols, err := getExchangeInfo(coinMFuturesURL); err == nil {
		assets := extractAssets(symbols, false) // Include only baseAsset (exclude quoteAsset, which is "USD")
		for k := range assets {
			allAssets[k] = struct{}{}
		}
	} else {
		fmt.Println("Error fetching COIN-M futures exchangeInfo:", err)
	}
	return allAssets
}

// ExchangeInfo represents the structure of the exchangeInfo API response
type ExchangeInfo struct {
	Symbols []Symbol `json:"symbols"`
}

// Symbol represents a trading symbol with its base and quote assets
type Symbol struct {
	BaseAsset  string `json:"baseAsset"`
	QuoteAsset string `json:"quoteAsset"`
}

// getExchangeInfo fetches the exchangeInfo from the given URL and returns the list of symbols
func getExchangeInfo(url string) ([]Symbol, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, url)
	}

	var info ExchangeInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	return info.Symbols, nil
}

// extractAssets extracts unique assets (baseAsset and optionally quoteAsset) from the list of symbols
func extractAssets(symbols []Symbol, includeQuote bool) map[string]bool {
	assets := make(map[string]bool)
	for _, symbol := range symbols {
		assets[symbol.BaseAsset] = true
		if includeQuote {
			assets[symbol.QuoteAsset] = true
		}
	}
	return assets
}
