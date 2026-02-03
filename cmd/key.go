package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const (
	ansiBrightGreen = "\033[92m"
	ansiBrightRed   = "\033[91m"
	ansiReset       = "\033[0m"
)

type apiErrorResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "检查所有 API Key 是否有效",
	Long:  "检查所有 API Key 是否有效。",
	Run: func(_ *cobra.Command, _ []string) {
		log.SetFlags(0)
		keys := requireMunaGeminiAPIKeys()
		if len(keys) == 0 {
			log.Fatal("missing MUNA_GEMINI_API_KEY")
		}

		ctx := context.Background()
		var wg sync.WaitGroup
		type result struct {
			key    string
			ok     bool
			reason string
		}
		results := make(chan result, len(keys))

		for _, key := range keys {
			key := key
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, reason := checkKey(ctx, key, keyTimeoutFlag)
				results <- result{key: key, ok: ok, reason: reason}
			}()
		}

		wg.Wait()
		close(results)

		hasFail := false
		for res := range results {
			masked := maskKey(res.key)
			if res.ok {
				fmt.Printf("%s     %sOK%s\n", masked, ansiBrightGreen, ansiReset)
				continue
			}
			hasFail = true
			if strings.TrimSpace(res.reason) == "" {
				fmt.Printf("%s     %sFAIL%s\n", masked, ansiBrightRed, ansiReset)
				continue
			}
			fmt.Printf("%s     %sFAIL%s %s\n", masked, ansiBrightRed, ansiReset, res.reason)
		}

		if hasFail {
			os.Exit(1)
		}
	},
}

func checkKey(ctx context.Context, key string, timeout time.Duration) (bool, string) {
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + key
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, ""
	}

	var apiErr apiErrorResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if apiErr.Error != nil {
		code := apiErr.Error.Code
		reason := ""
		for _, detail := range apiErr.Error.Details {
			if strings.TrimSpace(detail.Reason) != "" {
				reason = strings.TrimSpace(detail.Reason)
				break
			}
		}
		msg := strings.TrimSpace(apiErr.Error.Message)
		if code != 0 || reason != "" || msg != "" {
			parts := make([]string, 0, 3)
			if code != 0 {
				parts = append(parts, fmt.Sprintf("%d", code))
			}
			if reason != "" {
				parts = append(parts, reason)
			}
			if msg != "" {
				parts = append(parts, msg)
			}
			return false, strings.Join(parts, " ")
		}
		if apiErr.Error.Status != "" {
			return false, apiErr.Error.Status
		}
	}
	return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:4] + "..." + key[len(key)-8:]
}

var keyTimeoutFlag time.Duration

func init() {
	rootCmd.AddCommand(keyCmd)
	keyCmd.Flags().DurationVar(&keyTimeoutFlag, "timeout", 5*time.Second, "检查超时（例如 3s, 10s）")
}
