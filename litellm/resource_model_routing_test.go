package litellm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestModelRoutingUsesExplicitProviderOnce(t *testing.T) {
	for _, tc := range []struct{ provider, model, mode string }{
		{"chatgpt", "gpt-6-astra", "responses"},
		{"openai", "vendor/model", "chat"},
		{"openrouter", "openrouter/auto", "chat"},
		{"bedrock", "anthropic.claude-model", "chat"},
	} {
		for _, update := range []bool{false, true} {
			t.Run(tc.provider+map[bool]string{false: "/create", true: "/update"}[update], func(t *testing.T) {
				var request map[string]interface{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodPost {
						if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
							t.Errorf("decode request: %v", err)
						}
						_ = json.NewEncoder(w).Encode(request)
						return
					}
					_ = json.NewEncoder(w).Encode(s04WrappedModelResponse(request, r.URL.Query().Get("modelId")))
				}))
				defer server.Close()
				resource := resourceLiteLLMModel()
				data := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
					"model_name": "routing-canary", "custom_llm_provider": tc.provider,
					"base_model": tc.model, "mode": tc.mode,
				})
				if update {
					data.SetId("routing-canary-id")
				}
				if err := createOrUpdateModel(data, NewClient(server.URL, "test-key", false), update); err != nil {
					t.Fatal(err)
				}
				params := request["litellm_params"].(map[string]interface{})
				if params["model"] != tc.model || params["custom_llm_provider"] != tc.provider {
					t.Fatalf("routing must use bare model with explicit provider: %v", params)
				}
				if data.Get("base_model") != tc.model || data.Get("custom_llm_provider") != tc.provider {
					t.Fatal("routing identity changed during readback")
				}
			})
		}
	}
}
