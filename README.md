# muna-image-google

A Go CLI that generates images with the Gemini API (Nano Banana models).

## Requirements

- Go 1.22+
- A Gemini API key in your environment (e.g. `MUNA_GEMINI_API_KEY`)

## Install deps

```bash
go mod tidy
```

## Usage

```bash
# Generate with default prompt
MUNA_GEMINI_API_KEY=... go run .

# Custom prompt
MUNA_GEMINI_API_KEY=... go run . "A tiny robot painting a sunset" --out outputs

# Provide prompt via stdin
echo "A futuristic city skyline at dawn" | MUNA_GEMINI_API_KEY=... go run . --out outputs

# Select model
MUNA_GEMINI_API_KEY=... go run . --model gemini-3-pro-image-preview "A minimal logo for a tea shop" --out outputs

# Set aspect ratio and size (for gemini-3-pro-image-preview)
MUNA_GEMINI_API_KEY=... go run . "A modern cafe interior" --aspect 16:9 --size 2K --out outputs

# Increase total timeout
MUNA_GEMINI_API_KEY=... go run . "A modern cafe interior" --timeout 5m --out outputs

# Verbose HTTP logging (redacts API key)
MUNA_GEMINI_API_KEY=... go run . "A modern cafe interior" -v --out outputs
```

## Flags

```text
--model   Gemini image model ID (default: gemini-3-pro-image-preview)
--out     Output directory (default: .)
--aspect  Aspect ratio (e.g. 1:1, 16:9)
--size    Image size (1K, 2K, 4K for gemini-3-pro-image-preview) (default: 4K)
--timeout Total request timeout (e.g. 30s, 5m) (default: 5m)
--verbose Verbose HTTP logging (redacts API key, truncates large fields)
```

## Notes

- The default model is `gemini-3-pro-image-preview`.
