package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/genai"
)

const defaultPrompt = "Create a picture of a nano banana dish in a fancy restaurant with a Gemini theme."

var (
	modelFlag  string
	outFlag    string
	aspectFlag string
	sizeFlag   string
	timeoutFlag time.Duration
	verboseFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "muna-image-google [prompt]",
	Short: "Generate images with the Gemini API",
	Long: "Generate images with the Gemini API.\n" +
		"Provide the prompt as a positional argument, or via stdin.",
	Args: cobra.RangeArgs(0, 1),
	Run: func(_ *cobra.Command, args []string) {
		var text string
		if len(args) > 0 {
			text = strings.TrimSpace(args[0])
		}
		if text == "" {
			stdinText, err := readStdin()
			if err != nil {
				log.Fatal(err)
			}
			text = strings.TrimSpace(stdinText)
		}
		if text == "" {
			text = defaultPrompt
		}

		apiKey := requireMunaGeminiAPIKey()
		disableLocalGeminiBaseURL()

		ctx := context.Background()
		httpClient := &http.Client{Timeout: timeoutFlag}
		if verboseFlag {
			log.SetFlags(0)
			httpClient.Transport = &loggingTransport{
				base:   http.DefaultTransport,
				apiKey: apiKey,
			}
		}
		client, err := genai.NewClient(ctx, &genai.ClientConfig{HTTPClient: httpClient})
		if err != nil {
			log.Fatal(err)
		}

		var cfg *genai.GenerateContentConfig
		if aspectFlag != "" || sizeFlag != "" {
			cfg = &genai.GenerateContentConfig{
				ImageConfig: &genai.ImageConfig{
					AspectRatio: aspectFlag,
					ImageSize:   sizeFlag,
				},
			}
		}

		resp, err := client.Models.GenerateContent(ctx, modelFlag, genai.Text(text), cfg)
		if err != nil {
			log.Fatal(err)
		}

		imageBytes, mimeType, err := extractFirstImage(resp)
		if err != nil {
			log.Fatal(err)
		}

		outputPath, err := buildOutputPath(outFlag, mimeType)
		if err != nil {
			log.Fatal(err)
		}

		if err := os.WriteFile(outputPath, imageBytes, 0644); err != nil {
			log.Fatal(err)
		}
		absPath, err := filepath.Abs(outputPath)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(absPath)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&modelFlag, "model", "gemini-3-pro-image-preview", "Gemini image model ID")
	rootCmd.Flags().StringVar(&outFlag, "out", ".", "Output directory")
	rootCmd.Flags().StringVarP(&aspectFlag, "aspect", "a", "", "Aspect ratio (1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9)")
	rootCmd.Flags().StringVar(&sizeFlag, "size", "4K", "Image size (1K, 2K, 4K for gemini-3-pro-image-preview)")
	rootCmd.Flags().DurationVar(&timeoutFlag, "timeout", 5*time.Minute, "Total request timeout (e.g. 30s, 5m)")
	rootCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose HTTP logging (redacts API key, truncates large fields)")
}

func readStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}

	var b strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		b.WriteString(scanner.Text())
		b.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func extractFirstImage(resp *genai.GenerateContentResponse) ([]byte, string, error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, "", errors.New("no candidates returned")
	}

	var firstText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" && firstText == "" {
			firstText = part.Text
		}
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, part.InlineData.MIMEType, nil
		}
	}

	return nil, firstText, errors.New("no image data returned")
}

func buildOutputPath(outputDir, mimeType string) (string, error) {
	dir := strings.TrimSpace(outputDir)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	ext := extensionFromMIME(mimeType)
	filename := time.Now().Format("20060102") + randomUpperAlnum(12) + ext
	return filepath.Join(dir, filename), nil
}

func randomUpperAlnum(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func extensionFromMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func requireMunaGeminiAPIKey() string {
	key := strings.TrimSpace(os.Getenv("MUNA_GEMINI_API_KEY"))
	if key == "" {
		log.Fatal("missing MUNA_GEMINI_API_KEY")
	}
	_ = os.Setenv("GEMINI_API_KEY", key)
	return key
}

func disableLocalGeminiBaseURL() {
	if strings.TrimSpace(os.Getenv("GOOGLE_GEMINI_BASE_URL")) != "" {
		_ = os.Unsetenv("GOOGLE_GEMINI_BASE_URL")
	}
}

type loggingTransport struct {
	base   http.RoundTripper
	apiKey string
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}

	requestBody, err := readAndRestoreBody(req)
	if err != nil {
		return nil, err
	}
	logHTTP("REQUEST", req.Method, req.URL.String(), req.Header, requestBody, t.apiKey)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	responseBody, err := readAndRestoreBody(resp)
	if err != nil {
		return nil, err
	}
	logHTTP("RESPONSE", fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)), req.URL.String(), resp.Header, responseBody, t.apiKey)

	return resp, nil
}

func readAndRestoreBody(v interface{}) ([]byte, error) {
	switch r := v.(type) {
	case *http.Request:
		if r.Body == nil {
			return nil, nil
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		return body, nil
	case *http.Response:
		if r.Body == nil {
			return nil, nil
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		return body, nil
	default:
		return nil, nil
	}
}

func logHTTP(kind, statusOrMethod, url string, headers http.Header, body []byte, apiKey string) {
	sanitizedURL := redactString(url, apiKey)
	log.Printf("%s %s %s", kind, statusOrMethod, sanitizedURL)
	for k, v := range headers {
		if strings.EqualFold(k, "x-goog-api-key") || strings.EqualFold(k, "authorization") {
			log.Printf("%s: %s", k, "[REDACTED]")
			continue
		}
		log.Printf("%s: %s", k, strings.Join(v, ", "))
	}
	if len(body) == 0 {
		return
	}
	log.Println(formatBody(body, apiKey))
}

func formatBody(body []byte, apiKey string) string {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return truncateBytes(string(body), apiKey)
	}
	sanitizePayload(&payload, apiKey)
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return truncateBytes(string(body), apiKey)
	}
	return string(pretty)
}

func sanitizePayload(value *interface{}, apiKey string) {
	switch v := (*value).(type) {
	case map[string]interface{}:
		for k, val := range v {
			valCopy := val
			sanitizePayload(&valCopy, apiKey)
			v[k] = valCopy
		}
	case []interface{}:
		for i, val := range v {
			valCopy := val
			sanitizePayload(&valCopy, apiKey)
			v[i] = valCopy
		}
	case string:
		v = redactString(v, apiKey)
		if len([]byte(v)) > 1000 {
			v = truncateBytes(v, apiKey)
		}
		*value = v
	}
}

func redactString(s, apiKey string) string {
	if apiKey == "" {
		return s
	}
	return strings.ReplaceAll(s, apiKey, "[REDACTED]")
}

func truncateBytes(s, apiKey string) string {
	s = redactString(s, apiKey)
	b := []byte(s)
	if len(b) <= 1000 {
		return s
	}
	head := b[:500]
	tail := b[len(b)-500:]
	return fmt.Sprintf("%s...(%d bytes)...%s", string(head), len(b), string(tail))
}
