package cmd

import (
	"testing"

	"google.golang.org/genai"
)

func TestFilterModels_EmptyKeyword(t *testing.T) {
	models := []*genai.Model{{Name: "models/a"}, {Name: "models/b"}}
	got := filterModels(models, "")
	if len(got) != 2 {
		t.Fatalf("expected 2 models, got %d", len(got))
	}
}

func TestFilterModels_ByName(t *testing.T) {
	models := []*genai.Model{
		{Name: "models/gemini-image", DisplayName: "Gemini Image", Description: "desc"},
		{Name: "models/text-only", DisplayName: "Text", Description: "desc"},
	}
	got := filterModels(models, "image")
	if len(got) != 1 {
		t.Fatalf("expected 1 model, got %d", len(got))
	}
	if got[0].Name != "models/gemini-image" {
		t.Fatalf("unexpected model: %s", got[0].Name)
	}
}

func TestFilterModels_ByDisplayAndDescription_CaseInsensitive(t *testing.T) {
	models := []*genai.Model{
		{Name: "models/a", DisplayName: "Image Pro", Description: "high quality"},
		{Name: "models/b", DisplayName: "Other", Description: "for posters"},
	}

	got := filterModels(models, "QUALITY")
	if len(got) != 1 || got[0].Name != "models/a" {
		t.Fatalf("unexpected result for description filter: %+v", got)
	}

	got = filterModels(models, "post")
	if len(got) != 1 || got[0].Name != "models/b" {
		t.Fatalf("unexpected result for display/description filter: %+v", got)
	}
}

func TestFilterModels_SkipNilModel(t *testing.T) {
	models := []*genai.Model{nil, {Name: "models/gemini-image"}}
	got := filterModels(models, "gemini")
	if len(got) != 1 {
		t.Fatalf("expected 1 model, got %d", len(got))
	}
}

func TestContainsModelField(t *testing.T) {
	model := &genai.Model{Name: "models/gemini", DisplayName: "Gemini Pro", Description: "image generation"}
	if !containsModelField(model, "gemini") {
		t.Fatalf("expected match by name")
	}
	if !containsModelField(model, "pro") {
		t.Fatalf("expected match by display name")
	}
	if !containsModelField(model, "generation") {
		t.Fatalf("expected match by description")
	}
	if containsModelField(model, "unknown") {
		t.Fatalf("expected no match")
	}
}
