package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

func TestSplitAPIKeys_MixedDelimiters(t *testing.T) {
	raw := " key1,key2; key3\tkey4\nkey5\r\n"
	got := splitAPIKeys(raw)
	want := []string{"key1", "key2", "key3", "key4", "key5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected keys, got %v, want %v", got, want)
	}
}

func TestSplitAPIKeys_Empty(t *testing.T) {
	got := splitAPIKeys("")
	if len(got) != 0 {
		t.Fatalf("expected empty keys, got %v", got)
	}
}

func TestExtractMunaGeminiAPIKeysFromDotEnv(t *testing.T) {
	t.Run("line based format with comments", func(t *testing.T) {
		content := strings.Join([]string{
			"# account-a",
			"AIzaAAAAAAAAAAAAAAAAAAAAAAA1",
			"AIzaBBBBBBBBBBBBBBBBBBBBBBB2",
			"",
			"# account-b",
			"AIzaCCCCCCCCCCCCCCCCCCCCCCC3",
		}, "\n")

		got, suspicious := extractMunaGeminiAPIKeysFromDotEnv(content)
		want := []string{
			"AIzaAAAAAAAAAAAAAAAAAAAAAAA1",
			"AIzaBBBBBBBBBBBBBBBBBBBBBBB2",
			"AIzaCCCCCCCCCCCCCCCCCCCCCCC3",
		}
		if !reflect.DeepEqual(splitAPIKeys(got), want) {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() = %#v, want %#v", splitAPIKeys(got), want)
		}
		if len(suspicious) != 0 {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() suspicious = %#v, want empty", suspicious)
		}
	})

	t.Run("env assignment format should be supported", func(t *testing.T) {
		content := strings.Join([]string{
			"OTHER_KEY=abc",
			"MUNA_GEMINI_API_KEY=\"AIzaAAAAAAAAAAAAAAAAAAAAAAA1,AIzaBBBBBBBBBBBBBBBBBBBBBBB2\"",
			"export MUNA_GEMINI_API_KEY='AIzaCCCCCCCCCCCCCCCCCCCCCCC3 AIzaDDDDDDDDDDDDDDDDDDDDDDD4'",
			"MUNA_GEMINI_API_KEY=AIzaEEEEEEEEEEEEEEEEEEEEEEE5;AIzaFFFFFFFFFFFFFFFFFFFFFFF6 # with comment",
		}, "\n")

		got, suspicious := extractMunaGeminiAPIKeysFromDotEnv(content)
		want := []string{
			"AIzaAAAAAAAAAAAAAAAAAAAAAAA1",
			"AIzaBBBBBBBBBBBBBBBBBBBBBBB2",
			"AIzaCCCCCCCCCCCCCCCCCCCCCCC3",
			"AIzaDDDDDDDDDDDDDDDDDDDDDDD4",
			"AIzaEEEEEEEEEEEEEEEEEEEEEEE5",
			"AIzaFFFFFFFFFFFFFFFFFFFFFFF6",
		}
		if !reflect.DeepEqual(splitAPIKeys(got), want) {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() = %#v, want %#v", splitAPIKeys(got), want)
		}
		if len(suspicious) != 0 {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() suspicious = %#v, want empty", suspicious)
		}
	})

	t.Run("invalid lines should be marked suspicious", func(t *testing.T) {
		content := strings.Join([]string{
			"# keep",
			"not-a-key",
			"AIzaVALIDKEY123456789012345",
			"value with spaces",
			"MUNA_GEMINI_API_KEY=BAD1,BAD2",
		}, "\n")

		got, suspicious := extractMunaGeminiAPIKeysFromDotEnv(content)
		want := []string{"AIzaVALIDKEY123456789012345"}
		if !reflect.DeepEqual(splitAPIKeys(got), want) {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() = %#v, want %#v", splitAPIKeys(got), want)
		}
		if !reflect.DeepEqual(suspicious, []int{2, 4, 5}) {
			t.Fatalf("extractMunaGeminiAPIKeysFromDotEnv() suspicious = %#v, want %#v", suspicious, []int{2, 4, 5})
		}
	})
}

func TestLooksLikeGeminiAPIKeyAndFormatLineNumbers(t *testing.T) {
	if !looksLikeGeminiAPIKey("AIzaVALIDKEY123456789012345") {
		t.Fatal("looksLikeGeminiAPIKey() expected true")
	}
	if looksLikeGeminiAPIKey("short") {
		t.Fatal("looksLikeGeminiAPIKey() expected false for short value")
	}
	if looksLikeGeminiAPIKey("AIza INVALID KEY") {
		t.Fatal("looksLikeGeminiAPIKey() expected false for spaces")
	}
	got := formatLineNumbers([]int{2, 4, 9})
	if got != "2,4,9" {
		t.Fatalf("formatLineNumbers() = %q, want %q", got, "2,4,9")
	}
}

func TestLoadMunaGeminiAPIKeyRaw(t *testing.T) {
	t.Run("env should have higher priority than dot env", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("MUNA_GEMINI_API_KEY", "ENV_KEY_1,ENV_KEY_2")

		dir := filepath.Join(home, ".muna-image-google")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FILE_KEY_1\nFILE_KEY_2\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error: %v", err)
		}

		got, err := loadMunaGeminiAPIKeyRaw()
		if err != nil {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() unexpected error: %v", err)
		}
		want := []string{"ENV_KEY_1", "ENV_KEY_2"}
		if !reflect.DeepEqual(splitAPIKeys(got), want) {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() = %#v, want %#v", splitAPIKeys(got), want)
		}
	})

	t.Run("dot env should be used when env is empty", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("MUNA_GEMINI_API_KEY", "")

		dir := filepath.Join(home, ".muna-image-google")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll() error: %v", err)
		}
		content := strings.Join([]string{
			"# account-a",
			"AIzaFILEKEY123456789012345",
			"AIzaFILEKEY234567890123456",
		}, "\n")
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error: %v", err)
		}

		got, err := loadMunaGeminiAPIKeyRaw()
		if err != nil {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() unexpected error: %v", err)
		}
		want := []string{"AIzaFILEKEY123456789012345", "AIzaFILEKEY234567890123456"}
		if !reflect.DeepEqual(splitAPIKeys(got), want) {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() = %#v, want %#v", splitAPIKeys(got), want)
		}
	})

	t.Run("missing dot env should return empty", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("MUNA_GEMINI_API_KEY", "")

		got, err := loadMunaGeminiAPIKeyRaw()
		if err != nil {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("loadMunaGeminiAPIKeyRaw() = %q, want empty", got)
		}
	})
}

func TestCommandLongHelpIncludesKeySource(t *testing.T) {
	if !strings.Contains(rootCmd.Long, "MUNA_GEMINI_API_KEY") {
		t.Fatal("rootCmd.Long missing MUNA_GEMINI_API_KEY")
	}
	if !strings.Contains(rootCmd.Long, "~/.muna-image-google/.env") {
		t.Fatal("rootCmd.Long missing ~/.muna-image-google/.env")
	}
	if !strings.Contains(modelCmd.Long, "MUNA_GEMINI_API_KEY") {
		t.Fatal("modelCmd.Long missing MUNA_GEMINI_API_KEY")
	}
	if !strings.Contains(modelCmd.Long, "~/.muna-image-google/.env") {
		t.Fatal("modelCmd.Long missing ~/.muna-image-google/.env")
	}
	if !strings.Contains(keyCmd.Long, "MUNA_GEMINI_API_KEY") {
		t.Fatal("keyCmd.Long missing MUNA_GEMINI_API_KEY")
	}
	if !strings.Contains(keyCmd.Long, "~/.muna-image-google/.env") {
		t.Fatal("keyCmd.Long missing ~/.muna-image-google/.env")
	}
}

func TestResolveSeed_Specified(t *testing.T) {
	got, err := resolveSeed(true, 123)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 123 {
		t.Fatalf("expected seed 123, got %d", got)
	}
}

func TestResolveSeed_RandomRange(t *testing.T) {
	for i := 0; i < 5; i++ {
		got, err := resolveSeed(false, 0)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got < 0 || int64(got) > maxSeedValue {
			t.Fatalf("seed out of range: %d", got)
		}
	}
}

func TestExtensionFromMIME(t *testing.T) {
	cases := []struct {
		name string
		mime string
		want string
	}{
		{name: "png", mime: "image/png", want: ".png"},
		{name: "jpeg", mime: "image/jpeg", want: ".jpg"},
		{name: "jpg", mime: "image/jpg", want: ".jpg"},
		{name: "webp", mime: "image/webp", want: ".webp"},
		{name: "unknown", mime: "application/json", want: ".jpg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extensionFromMIME(tc.mime)
			if got != tc.want {
				t.Fatalf("unexpected extension, got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildDryRunConfigSnapshot(t *testing.T) {
	cfg := &genai.GenerateContentConfig{
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "16:9",
			ImageSize:   "2K",
		},
		SafetySettings: []*genai.SafetySetting{
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "OFF"},
			nil,
		},
	}

	got := buildDryRunConfigSnapshot(cfg, true, 42)
	if got["seed"] != int64(42) {
		t.Fatalf("unexpected seed: %v", got["seed"])
	}

	imageCfg, ok := got["imageConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected imageConfig map")
	}
	if imageCfg["aspectRatio"] != "16:9" || imageCfg["imageSize"] != "2K" {
		t.Fatalf("unexpected image config: %v", imageCfg)
	}

	safety, ok := got["safetySettings"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected safetySettings list")
	}
	if len(safety) != 1 {
		t.Fatalf("expected 1 safety setting, got %d", len(safety))
	}
}

func TestBuildDryRunConfigSnapshot_RandomSeed(t *testing.T) {
	got := buildDryRunConfigSnapshot(nil, false, 0)
	if len(got) != 0 {
		t.Fatalf("expected empty config for nil input, got %v", got)
	}

	cfg := &genai.GenerateContentConfig{}
	got = buildDryRunConfigSnapshot(cfg, false, 0)
	if got["seed"] != "random" {
		t.Fatalf("expected random seed marker, got %v", got["seed"])
	}
}

func TestBuildDryRunSnapshot(t *testing.T) {
	originalModel := modelFlag
	modelFlag = "test-model"
	defer func() { modelFlag = originalModel }()

	cfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	snapshot, err := buildDryRunSnapshot("hello", nil, cfg, false, 0, 3)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	execInfo, ok := snapshot["execution"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected execution map")
	}
	if execInfo["count"] != 3 {
		t.Fatalf("unexpected count: %v", execInfo["count"])
	}
	if execInfo["seedMode"] != "random-per-request" {
		t.Fatalf("unexpected seedMode: %v", execInfo["seedMode"])
	}

	request, ok := snapshot["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected request map")
	}
	if request["model"] != "test-model" {
		t.Fatalf("unexpected model: %v", request["model"])
	}
}

func TestExtractFirstImage_ReturnsImage(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "描述"},
						{InlineData: &genai.Blob{Data: []byte{1, 2, 3}, MIMEType: "image/png"}},
					},
				},
			},
		},
	}

	data, mimeType, err := extractFirstImage(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("unexpected mime type: %s", mimeType)
	}
	if !reflect.DeepEqual(data, []byte{1, 2, 3}) {
		t.Fatalf("unexpected image bytes: %v", data)
	}
}

func TestExtractFirstImage_NoImage(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{Parts: []*genai.Part{{Text: "只有文本"}}},
			},
		},
	}

	_, text, err := extractFirstImage(resp)
	if !errors.Is(err, errNoImage) {
		t.Fatalf("expected errNoImage, got %v", err)
	}
	if text != "只有文本" {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestExtractFinishMessage(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{FinishMessage: "来自响应"}},
	}
	if got := extractFinishMessage(resp, nil); got != "来自响应" {
		t.Fatalf("unexpected finish message: %s", got)
	}

	raw := []byte(`{"candidates":[{"finishMessage":"来自原始 JSON"}]}`)
	if got := extractFinishMessage(nil, raw); got != "来自原始 JSON" {
		t.Fatalf("unexpected finish message from raw: %s", got)
	}
}

func TestExtractFinishMessage_Empty(t *testing.T) {
	if got := extractFinishMessage(nil, nil); got != "" {
		t.Fatalf("expected empty message, got %s", got)
	}
}

func TestBuildParts_NoImages(t *testing.T) {
	parts, err := buildParts("hello", nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(parts) != 1 || parts[0].Text != "hello" {
		t.Fatalf("unexpected parts: %+v", parts)
	}
}

func TestBuildParts_TooManyImages(t *testing.T) {
	images := make([]string, 15)
	for i := range images {
		images[i] = "x.png"
	}

	_, err := buildParts("hello", images)
	if err == nil {
		t.Fatalf("expected error for too many images")
	}
}

func TestBuildParts_LocalImage(t *testing.T) {
	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "ref.png")
	imageBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if err := os.WriteFile(imagePath, imageBytes, 0644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	parts, err := buildParts("hello", []string{imagePath})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[1].InlineData == nil {
		t.Fatalf("expected inline data")
	}
	if !strings.HasPrefix(parts[1].InlineData.MIMEType, "image/png") {
		t.Fatalf("unexpected mime type: %s", parts[1].InlineData.MIMEType)
	}
}

func TestBuildParts_URLImage(t *testing.T) {
	originalTimeout := timeoutFlag
	timeoutFlag = 2 * time.Second
	defer func() { timeoutFlag = originalTimeout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	}))
	defer server.Close()

	parts, err := buildParts("hello", []string{server.URL + "/img"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[1].InlineData == nil {
		t.Fatalf("expected inline data")
	}
	if parts[1].InlineData.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mime type: %s", parts[1].InlineData.MIMEType)
	}
}

func TestBuildParts_URLNon2xx(t *testing.T) {
	originalTimeout := timeoutFlag
	timeoutFlag = 2 * time.Second
	defer func() { timeoutFlag = originalTimeout }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := buildParts("hello", []string{server.URL + "/not-found"})
	if err == nil {
		t.Fatalf("expected error for non-2xx response")
	}
}

func TestFetchURL_MIMEFallback(t *testing.T) {
	originalTimeout := timeoutFlag
	timeoutFlag = 2 * time.Second
	defer func() { timeoutFlag = originalTimeout }()

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	data, mimeType, err := fetchURL(server.URL)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !reflect.DeepEqual(data, pngBytes) {
		t.Fatalf("unexpected data: %v", data)
	}
	if !strings.HasPrefix(mimeType, "image/png") {
		t.Fatalf("unexpected mime type: %s", mimeType)
	}
}

func TestIsURL(t *testing.T) {
	if !isURL("https://example.com/a.png") {
		t.Fatalf("expected valid https url")
	}
	if !isURL("HTTP://example.com/a.png") {
		t.Fatalf("expected valid case-insensitive url")
	}
	if isURL("/tmp/a.png") {
		t.Fatalf("expected local path not be url")
	}
}

func TestReadAndRestoreBody_RequestAndResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("request-body"))
	body, err := readAndRestoreBody(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(body) != "request-body" {
		t.Fatalf("unexpected request body: %s", string(body))
	}
	restoredReqBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read restored request body: %v", err)
	}
	if string(restoredReqBody) != "request-body" {
		t.Fatalf("unexpected restored request body: %s", string(restoredReqBody))
	}

	resp := &http.Response{Body: io.NopCloser(strings.NewReader("response-body"))}
	body, err = readAndRestoreBody(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(body) != "response-body" {
		t.Fatalf("unexpected response body: %s", string(body))
	}
	restoredRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read restored response body: %v", err)
	}
	if string(restoredRespBody) != "response-body" {
		t.Fatalf("unexpected restored response body: %s", string(restoredRespBody))
	}
}

func TestBuildOutputPath(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "outputs")
	outputPath, err := buildOutputPath(outDir, "image/png", 42)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	base := filepath.Base(outputPath)
	pattern := regexp.MustCompile(`^\d{8}[A-Z0-9]{12}-42\.png$`)
	if !pattern.MatchString(base) {
		t.Fatalf("unexpected output filename: %s", base)
	}

	if _, err := os.Stat(outDir); err != nil {
		t.Fatalf("expected output dir created: %v", err)
	}
}

func TestPickRandomKey(t *testing.T) {
	if got := pickRandomKey([]string{"only"}); got != "only" {
		t.Fatalf("unexpected key: %s", got)
	}

	keys := []string{"a", "b", "c"}
	for i := 0; i < 10; i++ {
		got := pickRandomKey(keys)
		if got != "a" && got != "b" && got != "c" {
			t.Fatalf("selected key not in set: %s", got)
		}
	}
}

func TestDisableLocalGeminiBaseURL(t *testing.T) {
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "http://127.0.0.1:9999")
	disableLocalGeminiBaseURL()
	if got := os.Getenv("GOOGLE_GEMINI_BASE_URL"); got != "" {
		t.Fatalf("expected GOOGLE_GEMINI_BASE_URL unset, got %q", got)
	}
}

func TestRedactStringAndTruncateBytes(t *testing.T) {
	redacted := redactString("token=ABC123", "ABC123")
	if redacted != "token=[REDACTED]" {
		t.Fatalf("unexpected redacted string: %s", redacted)
	}

	long := strings.Repeat("x", 1200)
	truncated := truncateBytes(long, "")
	if !strings.Contains(truncated, "...(1200 bytes)...") {
		t.Fatalf("unexpected truncated marker: %s", truncated)
	}
}

func TestSanitizePayloadAndFormatBody(t *testing.T) {
	payload := interface{}(map[string]interface{}{
		"token": "ABC123",
		"nested": []interface{}{
			map[string]interface{}{"text": strings.Repeat("A", 1201)},
		},
	})
	sanitizePayload(&payload, "ABC123")

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, "ABC123") {
		t.Fatalf("expected api key redacted, got %s", text)
	}
	if !strings.Contains(text, "...(1201 bytes)...") {
		t.Fatalf("expected long string truncated, got %s", text)
	}

	formatted := formatBody([]byte(`{"apiKey":"ABC123"}`), "ABC123")
	if !strings.Contains(formatted, "[REDACTED]") {
		t.Fatalf("expected formatted body redacted, got %s", formatted)
	}

	nonJSON := formatBody([]byte("plain-ABC123-text"), "ABC123")
	if nonJSON != "plain-[REDACTED]-text" {
		t.Fatalf("unexpected non-json format output: %s", nonJSON)
	}
}

func TestResponseCapture_SetAndGet(t *testing.T) {
	var capture responseCapture

	small := []byte("hello")
	capture.set(small)
	got := capture.get()
	if string(got) != "hello" {
		t.Fatalf("unexpected capture payload: %s", string(got))
	}
	got[0] = 'H'
	if string(capture.get()) != "hello" {
		t.Fatalf("expected capture get returns copy")
	}

	big := bytes.Repeat([]byte{'a'}, maxCaptureBytes+10)
	capture.set(big)
	if len(capture.get()) != maxCaptureBytes {
		t.Fatalf("expected truncated capture size %d, got %d", maxCaptureBytes, len(capture.get()))
	}
}

func TestLoggingTransportRoundTrip(t *testing.T) {
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if string(requestBody) != `{"k":"v"}` {
			t.Fatalf("unexpected request body: %s", string(requestBody))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"token":"secret"}`)),
		}, nil
	})

	var buf bytes.Buffer
	originalLogWriter := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(originalLogWriter)

	req := httptest.NewRequest(http.MethodPost, "https://example.com/path?key=secret", strings.NewReader(`{"k":"v"}`))
	req.Header.Set("x-goog-api-key", "secret")

	transport := &loggingTransport{base: base, apiKey: "secret"}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(body) != `{"token":"secret"}` {
		t.Fatalf("unexpected response body: %s", string(body))
	}

	logged := buf.String()
	if !strings.Contains(logged, "REQUEST") || !strings.Contains(logged, "RESPONSE") {
		t.Fatalf("expected request and response logs, got %s", logged)
	}
	if !strings.Contains(logged, "[REDACTED]") {
		t.Fatalf("expected secrets redacted in logs, got %s", logged)
	}
}

func TestCaptureTransportRoundTrip(t *testing.T) {
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"finishMessage":"done"}`)),
		}, nil
	})

	capture := &responseCapture{}
	transport := &captureTransport{base: base, apiKey: "secret", verbose: false, capture: capture}
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer resp.Body.Close()

	if string(capture.get()) != `{"finishMessage":"done"}` {
		t.Fatalf("unexpected captured body: %s", string(capture.get()))
	}
}

func TestLoggingTransportRoundTrip_DefaultTransport(t *testing.T) {
	originalDefaultTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})
	defer func() { http.DefaultTransport = originalDefaultTransport }()

	transport := &loggingTransport{apiKey: "abc"}
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
}

func TestCaptureTransportRoundTrip_VerboseAndDefaultTransport(t *testing.T) {
	originalDefaultTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})
	defer func() { http.DefaultTransport = originalDefaultTransport }()

	var buf bytes.Buffer
	originalLogWriter := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(originalLogWriter)

	transport := &captureTransport{apiKey: "abc", verbose: true, capture: &responseCapture{}}
	req := httptest.NewRequest(http.MethodPost, "https://example.com", strings.NewReader(`{"k":"v"}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer resp.Body.Close()

	if !strings.Contains(buf.String(), "REQUEST") || !strings.Contains(buf.String(), "RESPONSE") {
		t.Fatalf("expected verbose logs, got %s", buf.String())
	}
}

func TestReadStdin_FromPipe(t *testing.T) {
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	tempFile, err := os.CreateTemp(t.TempDir(), "stdin-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp stdin: %v", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString("line1\nline2\n"); err != nil {
		t.Fatalf("failed to write stdin: %v", err)
	}
	if _, err := tempFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to seek stdin file: %v", err)
	}
	os.Stdin = tempFile

	got, err := readStdin()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "line1\nline2\n" {
		t.Fatalf("unexpected stdin content: %q", got)
	}
}

func TestDetectMIME(t *testing.T) {
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if got := detectMIME("a.png", []byte("abc")); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("expected extension mime image/png, got %s", got)
	}

	if got := detectMIME("", pngBytes); !strings.HasPrefix(got, "image/png") {
		t.Fatalf("expected sniffed image/png, got %s", got)
	}

	large := bytes.Repeat([]byte{'a'}, 1024)
	if got := detectMIME("", large); got == "" {
		t.Fatalf("expected non-empty mime for large payload")
	}
}

func TestBuildOutputPath_EmptyDirAndMkdirError(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })

	pathInCWD, err := buildOutputPath("", "image/jpeg", 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if filepath.Dir(pathInCWD) != "." {
		t.Fatalf("expected relative path in current directory, got %s", pathInCWD)
	}

	fileAsDir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := buildOutputPath(fileAsDir, "image/png", 1); err == nil {
		t.Fatalf("expected mkdir error when output path is a file")
	}
}

func TestReadAndRestoreBody_NilAndDefault(t *testing.T) {
	req := &http.Request{}
	body, err := readAndRestoreBody(req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil body, got %v", body)
	}

	resp := &http.Response{}
	body, err = readAndRestoreBody(resp)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil response body, got %v", body)
	}

	body, err = readAndRestoreBody(struct{}{})
	if err != nil {
		t.Fatalf("expected nil error for default branch, got %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil default body, got %v", body)
	}
}

func TestFetchURL_InvalidContentTypeHeaderFallback(t *testing.T) {
	originalTimeout := timeoutFlag
	timeoutFlag = 2 * time.Second
	defer func() { timeoutFlag = originalTimeout }()

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "invalid/;")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	_, mimeType, err := fetchURL(server.URL)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.HasPrefix(mimeType, "image/png") {
		t.Fatalf("expected fallback mime image/png, got %s", mimeType)
	}
}

func TestRandomUpperAlnum(t *testing.T) {
	got := randomUpperAlnum(64)
	if len(got) != 64 {
		t.Fatalf("expected length 64, got %d", len(got))
	}
	pattern := regexp.MustCompile(`^[A-Z0-9]+$`)
	if !pattern.MatchString(got) {
		t.Fatalf("unexpected random string: %s", got)
	}
}

func TestRootCmdDryRun_WithArgPrompt(t *testing.T) {
	t.Setenv("MUNA_GEMINI_API_KEY", "k1,k2")

	stdout, stderr, err := runRootCommandForTest(t, []string{"--dry-run", "--count", "2", "--seed", "42", "hello world"}, "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got %s", stderr)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("invalid dry-run json output: %v", err)
	}

	execInfo, ok := payload["execution"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected execution map")
	}
	if execInfo["seedMode"] != "fixed" {
		t.Fatalf("unexpected seed mode: %v", execInfo["seedMode"])
	}
	if execInfo["count"].(float64) != 2 {
		t.Fatalf("unexpected count: %v", execInfo["count"])
	}
}

func TestRootCmdDryRun_WithStdinPrompt(t *testing.T) {
	t.Setenv("MUNA_GEMINI_API_KEY", "k1")

	stdout, _, err := runRootCommandForTest(t, []string{"--dry-run"}, "prompt-from-stdin\n")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(stdout, "prompt-from-stdin") {
		t.Fatalf("expected prompt text in output, got %s", stdout)
	}
}

func runRootCommandForTest(t *testing.T, args []string, stdinContent string) (string, string, error) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalStdin := os.Stdin
	originalModel := modelFlag
	originalOut := outFlag
	originalAspect := aspectFlag
	originalSize := sizeFlag
	originalTimeout := timeoutFlag
	originalVerbose := verboseFlag
	originalDryRun := dryRunFlag
	originalKeyPatterns := append([]string(nil), keyPatterns...)
	originalRefPaths := append([]string(nil), refPaths...)
	originalCount := countFlag
	originalSeed := seedFlag
	originalModelJSON := modelJSONFlag
	originalKeyTimeout := keyTimeoutFlag
	originalSilenceErrors := rootCmd.SilenceErrors
	originalSilenceUsage := rootCmd.SilenceUsage

	t.Cleanup(func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		os.Stdin = originalStdin
		modelFlag = originalModel
		outFlag = originalOut
		aspectFlag = originalAspect
		sizeFlag = originalSize
		timeoutFlag = originalTimeout
		verboseFlag = originalVerbose
		dryRunFlag = originalDryRun
		keyPatterns = originalKeyPatterns
		refPaths = originalRefPaths
		countFlag = originalCount
		seedFlag = originalSeed
		modelJSONFlag = originalModelJSON
		keyTimeoutFlag = originalKeyTimeout
		rootCmd.SilenceErrors = originalSilenceErrors
		rootCmd.SilenceUsage = originalSilenceUsage
		rootCmd.SetArgs(nil)
	})

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	if stdinContent != "" {
		stdinFile, err := os.CreateTemp(t.TempDir(), "stdin-*.txt")
		if err != nil {
			return "", "", err
		}
		if _, err := stdinFile.WriteString(stdinContent); err != nil {
			return "", "", err
		}
		if _, err := stdinFile.Seek(0, 0); err != nil {
			return "", "", err
		}
		os.Stdin = stdinFile
	}

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)
	execErr := rootCmd.Execute()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		return "", "", err
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		return "", "", err
	}

	return string(stdoutBytes), string(stderrBytes), execErr
}
