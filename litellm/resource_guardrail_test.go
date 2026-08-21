package litellm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestGuardrailResourceLifecycle(t *testing.T) {
	var requests []string
	var stored map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var payload struct {
				Guardrail map[string]interface{} `json:"guardrail"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			stored = payload.Guardrail
			response := copyGuardrail(stored)
			response["guardrail_id"] = "guardrail-1"
			json.NewEncoder(w).Encode(response)
		case http.MethodGet:
			response := copyGuardrail(stored)
			response["guardrail_id"] = "guardrail-1"
			params := copyGuardrail(response["litellm_params"].(map[string]interface{}))
			params["api_key"] = "****cret"
			response["litellm_params"] = params
			json.NewEncoder(w).Encode(response)
		case http.MethodDelete:
			json.NewEncoder(w).Encode(map[string]string{"guardrail_id": "guardrail-1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	resource, ok := Provider().ResourcesMap["litellm_guardrail"]
	if !ok {
		t.Fatal("litellm_guardrail is not registered")
	}
	if resource.Importer == nil {
		t.Fatal("litellm_guardrail has no importer")
	}
	if !resource.Schema["litellm_params"].Sensitive {
		t.Fatal("litellm_params must be sensitive")
	}

	params := `{"guardrail":"generic_guardrail_api","mode":["pre_call","post_call"],"api_key":"secret","additional_provider_specific_params":{"threshold":0.8}}`
	info := `{"description":"protect requests"}`
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		"guardrail_name": "request-filter",
		"litellm_params": params,
		"guardrail_info": info,
	})
	client := NewClient(server.URL, "test-key", false)

	if err := resource.Create(d, client); err != nil {
		t.Fatalf("create guardrail: %v", err)
	}
	if d.Id() != "guardrail-1" {
		t.Fatalf("resource ID = %q, want guardrail-1", d.Id())
	}
	assertJSONEqual(t, d.Get("litellm_params").(string), params)

	updatedParams := `{"guardrail":"generic_guardrail_api","mode":"pre_call","api_key":"new-secret","timeout":3}`
	updatedInfo := `{"description":"protect all requests"}`
	if err := d.Set("guardrail_name", "request-filter-v2"); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("litellm_params", updatedParams); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("guardrail_info", updatedInfo); err != nil {
		t.Fatal(err)
	}
	if err := resource.Update(d, client); err != nil {
		t.Fatalf("update guardrail: %v", err)
	}
	assertJSONEqual(t, d.Get("litellm_params").(string), updatedParams)
	assertJSONEqual(t, d.Get("guardrail_info").(string), updatedInfo)

	if err := resource.Delete(d, client); err != nil {
		t.Fatalf("delete guardrail: %v", err)
	}
	if d.Id() != "" {
		t.Fatalf("resource ID after delete = %q, want empty", d.Id())
	}

	wantRequests := []string{
		"POST /guardrails",
		"GET /guardrails/guardrail-1",
		"PUT /guardrails/guardrail-1",
		"GET /guardrails/guardrail-1",
		"DELETE /guardrails/guardrail-1",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestGuardrailReadClearsMissingResource(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	resource := Provider().ResourcesMap["litellm_guardrail"]
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{})
	d.SetId("missing")
	if err := resource.Read(d, NewClient(server.URL, "test-key", false)); err != nil {
		t.Fatalf("read missing guardrail: %v", err)
	}
	if d.Id() != "" {
		t.Fatalf("resource ID = %q, want empty", d.Id())
	}
}

func TestGuardrailImportReadsMaskedDefinition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"guardrail_id":   "guardrail-1",
			"guardrail_name": "imported",
			"litellm_params": map[string]interface{}{
				"guardrail": "generic_guardrail_api",
				"mode":      "pre_call",
				"api_key":   "****cret",
			},
			"guardrail_info": map[string]interface{}{"description": "imported definition"},
		})
	}))
	defer server.Close()

	resource := Provider().ResourcesMap["litellm_guardrail"]
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{})
	d.SetId("guardrail-1")
	if err := resource.Read(d, NewClient(server.URL, "test-key", false)); err != nil {
		t.Fatalf("read imported guardrail: %v", err)
	}
	assertJSONEqual(t, d.Get("litellm_params").(string), `{"guardrail":"generic_guardrail_api","mode":"pre_call","api_key":"****cret"}`)
}

func TestGuardrailCreateRedactsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail":"invalid api_key secret"}`))
	}))
	defer server.Close()

	resource := Provider().ResourcesMap["litellm_guardrail"]
	d := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		"guardrail_name": "broken",
		"litellm_params": `{"guardrail":"generic_guardrail_api","mode":"pre_call","api_key":"secret"}`,
	})
	err := resource.Create(d, NewClient(server.URL, "test-key", false))
	if err == nil {
		t.Fatal("create guardrail unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func copyGuardrail(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotValue, wantValue interface{}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("invalid actual JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("invalid expected JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}
