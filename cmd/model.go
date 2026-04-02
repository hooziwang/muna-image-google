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
	Long: "列出服务端模型，支持关键词模糊查询。\n" +
		"环境变量：MUNA_IMAGE_GOOGLE_BASE_URL、MUNA_IMAGE_GOOGLE_API_KEY。\n" +
		"仅支持 Google 官方默认地址 https://generativelanguage.googleapis.com；当 MUNA_IMAGE_GOOGLE_BASE_URL 被设置为非官方地址时会直接退出。\n" +
		"Key 回退来源：未设置 MUNA_IMAGE_GOOGLE_API_KEY 时，继续读取 MUNA_GEMINI_API_KEY；仍未设置时回退 ~/.muna-image-google/.env。",
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runModelCommand(cmd, args); err != nil {
			log.SetFlags(0)
			log.Fatal(err)
		}
	},
}

func runModelCommand(_ *cobra.Command, args []string) error {
	log.SetFlags(0)
	if err := validateOfficialBaseURLForMetaCommands(); err != nil {
		return err
	}

	keys := requireMunaGeminiAPIKeys()
	if len(keys) == 0 {
		return fmt.Errorf("缺少环境变量 MUNA_IMAGE_GOOGLE_API_KEY、MUNA_GEMINI_API_KEY（或 ~/.muna-image-google/.env）")
	}

	query := ""
	if len(args) > 0 {
		query = strings.TrimSpace(args[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutFlag)
	defer cancel()

	client, err := newGenAIClient(ctx, pickRandomKey(keys), &http.Client{Timeout: timeoutFlag})
	if err != nil {
		return err
	}

	models, err := fetchAllModels(ctx, client)
	if err != nil {
		return err
	}
	models = filterModels(models, query)

	if modelJSONFlag {
		data, err := json.MarshalIndent(models, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(models) == 0 {
		fmt.Println("未匹配到模型")
		return nil
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

	return nil
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
