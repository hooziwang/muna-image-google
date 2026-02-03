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
	"math/big"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	refPaths []string
	countFlag int
)

const maxCaptureBytes = 2 << 20

var errNoImage = errors.New("no image data returned")

var rootCmd = &cobra.Command{
	Use:   "muna-image-google [prompt]",
	Short: "使用 Gemini API 生成图像",
	Long: "使用 Gemini API 生成图像。\n" +
		"提示词可用位置参数提供，也可从标准输入读取。",
	Args: cobra.RangeArgs(0, 1),
	Run: func(_ *cobra.Command, args []string) {
		log.SetFlags(0)
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

		apiKeys := requireMunaGeminiAPIKeys()
		disableLocalGeminiBaseURL()

		cfg := &genai.GenerateContentConfig{
			SafetySettings: defaultSafetySettings(),
		}
		if aspectFlag != "" || sizeFlag != "" {
			cfg.ImageConfig = &genai.ImageConfig{
				AspectRatio: aspectFlag,
				ImageSize:   sizeFlag,
			}
		}

		if countFlag < 1 {
			log.Fatal("count must be >= 1")
		}

		ctx := context.Background()
		var wg sync.WaitGroup
		var outputMu sync.Mutex
		errCh := make(chan error, countFlag)

		for i := 0; i < countFlag; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				apiKey := pickRandomKey(apiKeys)
				absPath, finishMessage, err := generateOnce(ctx, apiKey, text, refPaths, cfg)
				if err != nil {
					if errors.Is(err, errNoImage) {
						if !verboseFlag && finishMessage != "" {
							outputMu.Lock()
							fmt.Fprintln(os.Stderr, finishMessage)
							outputMu.Unlock()
						}
						errCh <- err
						return
					}
					errCh <- err
					return
				}
				outputMu.Lock()
				fmt.Println(absPath)
				outputMu.Unlock()
			}()
		}

		wg.Wait()
		close(errCh)
		if len(errCh) > 0 {
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&modelFlag, "model", "m", "gemini-3-pro-image-preview", "模型 ID")
	rootCmd.Flags().StringVarP(&outFlag, "out", "o", ".", "输出目录")
	rootCmd.Flags().StringVarP(&aspectFlag, "aspect", "a", "", "宽高比（1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9）")
	rootCmd.Flags().StringVar(&sizeFlag, "size", "4K", "图像尺寸（1K, 2K, 4K，适用于 gemini-3-pro-image-preview）")
	rootCmd.Flags().DurationVar(&timeoutFlag, "timeout", 5*time.Minute, "总超时（例如 30s, 5m）")
	rootCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "详细日志（脱敏 API Key，长字段裁剪）")
	rootCmd.Flags().StringArrayVarP(&refPaths, "ref", "r", nil, "参考图片路径（可重复，最多 14 张）")
	rootCmd.Flags().IntVarP(&countFlag, "count", "n", 1, "生成数量")
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

	return nil, firstText, errNoImage
}

func extractFinishMessage(resp *genai.GenerateContentResponse, raw []byte) string {
	if resp != nil && len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		if msg := strings.TrimSpace(resp.Candidates[0].FinishMessage); msg != "" {
			return msg
		}
	}
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	candidates, ok := payload["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return ""
	}
	first, ok := candidates[0].(map[string]interface{})
	if !ok {
		return ""
	}
	if msg, ok := first["finishMessage"].(string); ok {
		return strings.TrimSpace(msg)
	}
	return ""
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

func requireMunaGeminiAPIKeys() []string {
	raw := strings.TrimSpace(os.Getenv("MUNA_GEMINI_API_KEY"))
	keys := splitAPIKeys(raw)
	if len(keys) == 0 {
		log.Fatal("missing MUNA_GEMINI_API_KEY")
	}
	return keys
}

func splitAPIKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func pickRandomKey(keys []string) string {
	if len(keys) == 1 {
		return keys[0]
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(keys))))
	if err != nil {
		return keys[0]
	}
	return keys[n.Int64()]
}

func disableLocalGeminiBaseURL() {
	if strings.TrimSpace(os.Getenv("GOOGLE_GEMINI_BASE_URL")) != "" {
		_ = os.Unsetenv("GOOGLE_GEMINI_BASE_URL")
	}
}

func defaultSafetySettings() []*genai.SafetySetting {
	return []*genai.SafetySetting{
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "OFF"},
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "OFF"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "OFF"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "OFF"},
		{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "OFF"},
	}
}

func buildParts(text string, images []string) ([]*genai.Part, error) {
	parts := []*genai.Part{{Text: text}}
	if len(images) == 0 {
		return parts, nil
	}
	if len(images) > 14 {
		return nil, fmt.Errorf("too many reference images: %d (max 14)", len(images))
	}
	for _, path := range images {
		var data []byte
		var mimeType string
		var err error
		if isURL(path) {
			data, mimeType, err = fetchURL(path)
		} else {
			data, err = os.ReadFile(path)
			if err == nil {
				mimeType = detectMIME(path, data)
			}
		}
		if err != nil {
			return nil, err
		}
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				Data:     data,
				MIMEType: mimeType,
			},
		})
	}
	return parts, nil
}

func detectMIME(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if m := mime.TypeByExtension(ext); m != "" {
			return m
		}
	}
	if len(data) >= 512 {
		return http.DetectContentType(data[:512])
	}
	return http.DetectContentType(data)
}

func isURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func fetchURL(rawURL string) ([]byte, string, error) {
	client := &http.Client{Timeout: timeoutFlag}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("failed to download %s: %s", rawURL, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mimeType := ""
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if mediaType, _, err := mime.ParseMediaType(ct); err == nil {
			mimeType = mediaType
		}
	}
	if mimeType == "" {
		mimeType = detectMIME("", data)
	}
	return data, mimeType, nil
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

type captureTransport struct {
	base    http.RoundTripper
	apiKey  string
	verbose bool
	capture *responseCapture
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}

	if t.verbose {
		requestBody, err := readAndRestoreBody(req)
		if err != nil {
			return nil, err
		}
		logHTTP("REQUEST", req.Method, req.URL.String(), req.Header, requestBody, t.apiKey)
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	responseBody, err := readAndRestoreBody(resp)
	if err != nil {
		return nil, err
	}
	if t.capture != nil {
		t.capture.set(responseBody)
	}
	if t.verbose {
		logHTTP("RESPONSE", fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)), req.URL.String(), resp.Header, responseBody, t.apiKey)
	}

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

type responseCapture struct {
	mu   sync.Mutex
	body []byte
}

func (c *responseCapture) set(body []byte) {
	if len(body) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(body) > maxCaptureBytes {
		c.body = append([]byte(nil), body[:maxCaptureBytes]...)
		return
	}
	c.body = append([]byte(nil), body...)
}

func (c *responseCapture) get() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.body...)
}

func generateOnce(ctx context.Context, apiKey, text string, refs []string, cfg *genai.GenerateContentConfig) (string, string, error) {
	capture := &responseCapture{}
	httpClient := &http.Client{Timeout: timeoutFlag}
	httpClient.Transport = &captureTransport{
		base:    http.DefaultTransport,
		apiKey:  apiKey,
		verbose: verboseFlag,
		capture: capture,
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		HTTPClient: httpClient,
		APIKey:     apiKey,
	})
	if err != nil {
		return "", "", err
	}

	parts, err := buildParts(text, refs)
	if err != nil {
		return "", "", err
	}
	contents := []*genai.Content{{Parts: parts}}

	resp, err := client.Models.GenerateContent(ctx, modelFlag, contents, cfg)
	if err != nil {
		return "", "", err
	}

	imageBytes, mimeType, err := extractFirstImage(resp)
	if err != nil {
		if errors.Is(err, errNoImage) {
			return "", extractFinishMessage(resp, capture.get()), errNoImage
		}
		return "", "", err
	}

	outputPath, err := buildOutputPath(outFlag, mimeType)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(outputPath, imageBytes, 0644); err != nil {
		return "", "", err
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", "", err
	}
	return absPath, "", nil
}
