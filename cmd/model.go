package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/genai"
)

var modelJSONFlag bool

var modelCmd = &cobra.Command{
	Use:   "model [keyword]",
	Short: "列出服务端模型",
	Long:  "列出服务端模型，支持关键词模糊查询。",
	Args:  cobra.RangeArgs(0, 1),
	Run: func(_ *cobra.Command, args []string) {
		log.SetFlags(0)
		keys := requireMunaGeminiAPIKeys()
		if len(keys) == 0 {
			log.Fatal("缺少环境变量 MUNA_GEMINI_API_KEY")
		}
		disableLocalGeminiBaseURL()

		query := ""
		if len(args) > 0 {
			query = strings.TrimSpace(args[0])
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeoutFlag)
		defer cancel()

		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			HTTPClient: &http.Client{Timeout: timeoutFlag},
			APIKey:     pickRandomKey(keys),
		})
		if err != nil {
			log.Fatal(err)
		}

		models, err := fetchAllModels(ctx, client)
		if err != nil {
			log.Fatal(err)
		}
		models = filterModels(models, query)

		if modelJSONFlag {
			data, err := json.MarshalIndent(models, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(data))
			return
		}

		if len(models) == 0 {
			fmt.Println("未匹配到模型")
			return
		}

		type modelRow struct {
			name    string
			display string
			desc    string
		}

		rows := make([]modelRow, 0, len(models))
		for _, model := range models {
			if model == nil {
				continue
			}
			name := strings.TrimSpace(model.Name)
			display := strings.TrimSpace(model.DisplayName)
			desc := strings.TrimSpace(model.Description)
			if name == "" {
				name = "-"
			}
			if display == "" {
				display = "-"
			}
			if desc == "" {
				desc = "-"
			}
			rows = append(rows, modelRow{
				name:    name,
				display: display,
				desc:    desc,
			})
		}

		for _, row := range rows {
			fmt.Printf("%s : %s\n", row.name, row.display)
			fmt.Printf("\033[90m%s\033[0m\n", row.desc)
		}
	},
}

func fetchAllModels(ctx context.Context, client *genai.Client) ([]*genai.Model, error) {
	page, err := client.Models.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	var models []*genai.Model
	for {
		models = append(models, page.Items...)
		next, err := page.Next(ctx)
		if err == genai.ErrPageDone {
			break
		}
		if err != nil {
			return nil, err
		}
		page = next
	}

	return models, nil
}

func filterModels(models []*genai.Model, keyword string) []*genai.Model {
	trimmed := strings.TrimSpace(keyword)
	if trimmed == "" {
		return models
	}

	query := strings.ToLower(trimmed)
	out := make([]*genai.Model, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		if containsModelField(model, query) {
			out = append(out, model)
		}
	}
	return out
}

func containsModelField(model *genai.Model, query string) bool {
	return strings.Contains(strings.ToLower(model.Name), query) ||
		strings.Contains(strings.ToLower(model.DisplayName), query) ||
		strings.Contains(strings.ToLower(model.Description), query)
}

func init() {
	rootCmd.AddCommand(modelCmd)
	modelCmd.Flags().BoolVar(&modelJSONFlag, "json", false, "以 JSON 输出完整模型信息")
}

