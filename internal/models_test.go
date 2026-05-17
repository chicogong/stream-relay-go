package internal

import "testing"

func TestExtractProvider(t *testing.T) {
	cases := map[string]string{
		"https://api.openai.com":                    "openai",
		"https://api.anthropic.com":                 "anthropic",
		"https://api.siliconflow.cn":                "siliconflow",
		"https://eastus.tts.speech.microsoft.com":   "azure",
		"https://generativelanguage.googleapis.com": "google",
		"https://api.deepseek.com":                  "deepseek",
		"https://api.example.com":                   "example",
		"":                                          "unknown",
		"://bad-url":                                "unknown",
	}
	for upstream, want := range cases {
		if got := extractProvider(upstream); got != want {
			t.Errorf("extractProvider(%q) = %q, want %q", upstream, got, want)
		}
	}
}

func TestExtractModel(t *testing.T) {
	cases := map[string]string{
		`{"model":"gpt-4o","messages":[]}`:     "gpt-4o",
		`{"messages":[],"model":"claude-3-5"}`: "claude-3-5",
		`{"messages":[]}`:                      "",
		`not json`:                             "",
		``:                                     "",
	}
	for body, want := range cases {
		if got := extractModel(body); got != want {
			t.Errorf("extractModel(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestExtractUsage_OpenAI(t *testing.T) {
	chunks := []string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`,
		`data: [DONE]`,
	}
	u := extractUsage(chunks)
	if u == nil {
		t.Fatal("expected usage, got nil")
	}
	if u.InputTokens != 12 || u.OutputTokens != 34 || u.TotalTokens != 46 {
		t.Errorf("got %+v, want {12 34 46}", *u)
	}
}

func TestExtractUsage_Anthropic(t *testing.T) {
	// Anthropic 把 usage 分散在 message_start 与 message_delta 事件中
	chunks := []string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":25,"output_tokens":1}}}`,
		`data: {"type":"content_block_delta","delta":{"text":"hello"}}`,
		`data: {"type":"message_delta","usage":{"output_tokens":40}}`,
	}
	u := extractUsage(chunks)
	if u == nil {
		t.Fatal("expected usage, got nil")
	}
	if u.InputTokens != 25 || u.OutputTokens != 40 {
		t.Errorf("got %+v, want input=25 output=40", *u)
	}
	if u.TotalTokens != 65 {
		t.Errorf("TotalTokens = %d, want 65 (derived)", u.TotalTokens)
	}
}

func TestExtractUsage_None(t *testing.T) {
	chunks := []string{
		`data: {"choices":[{"delta":{"content":"hi"}}]}`,
		`data: [DONE]`,
		`: comment line`,
		``,
	}
	if u := extractUsage(chunks); u != nil {
		t.Errorf("expected nil usage, got %+v", *u)
	}
}

func TestStatusClass(t *testing.T) {
	cases := map[int]string{
		0: "5xx", 200: "2xx", 204: "2xx",
		400: "4xx", 404: "4xx", 429: "4xx",
		500: "5xx", 502: "5xx",
	}
	for code, want := range cases {
		if got := statusClass(code); got != want {
			t.Errorf("statusClass(%d) = %q, want %q", code, got, want)
		}
	}
}
