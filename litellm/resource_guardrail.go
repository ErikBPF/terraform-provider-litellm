package litellm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceLiteLLMGuardrail() *schema.Resource {
	return &schema.Resource{
		Create: resourceLiteLLMGuardrailCreate,
		Read:   resourceLiteLLMGuardrailRead,
		Update: resourceLiteLLMGuardrailUpdate,
		Delete: resourceLiteLLMGuardrailDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"guardrail_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"litellm_params": {
				Type:             schema.TypeString,
				Required:         true,
				Sensitive:        true,
				ValidateFunc:     validation.StringIsJSON,
				StateFunc:        normalizeGuardrailJSON,
				DiffSuppressFunc: structure.SuppressJsonDiff,
			},
			"guardrail_info": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "{}",
				ValidateFunc:     validation.StringIsJSON,
				StateFunc:        normalizeGuardrailJSON,
				DiffSuppressFunc: structure.SuppressJsonDiff,
			},
		},
	}
}

func normalizeGuardrailJSON(value interface{}) string {
	normalized, err := structure.NormalizeJsonString(value)
	if err != nil {
		return value.(string)
	}
	return normalized
}

func guardrailPayload(d *schema.ResourceData) (map[string]interface{}, error) {
	var params, info map[string]interface{}
	if err := json.Unmarshal([]byte(d.Get("litellm_params").(string)), &params); err != nil {
		return nil, fmt.Errorf("invalid litellm_params: %w", err)
	}
	if err := json.Unmarshal([]byte(d.Get("guardrail_info").(string)), &info); err != nil {
		return nil, fmt.Errorf("invalid guardrail_info: %w", err)
	}
	return map[string]interface{}{
		"guardrail": map[string]interface{}{
			"guardrail_name": d.Get("guardrail_name").(string),
			"litellm_params": params,
			"guardrail_info": info,
		},
	}, nil
}

func sendGuardrailRequest(client *Client, method, path string, payload interface{}) (map[string]interface{}, error) {
	resp, err := MakeRequest(client, method, path, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("API request failed with status code %d", resp.StatusCode)
	}
	if len(bytes.TrimSpace(body)) == 0 || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

func resourceLiteLLMGuardrailCreate(d *schema.ResourceData, m interface{}) error {
	payload, err := guardrailPayload(d)
	if err != nil {
		return err
	}
	result, err := sendGuardrailRequest(m.(*Client), http.MethodPost, "/guardrails", payload)
	if err != nil {
		return fmt.Errorf("failed to create guardrail: %w", err)
	}
	id, _ := result["guardrail_id"].(string)
	if id == "" {
		return fmt.Errorf("guardrail create response omitted guardrail_id")
	}
	d.SetId(id)
	return resourceLiteLLMGuardrailRead(d, m)
}

func resourceLiteLLMGuardrailRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	resp, err := MakeRequest(client, http.MethodGet, "/guardrails/"+d.Id(), nil)
	if err != nil {
		return fmt.Errorf("failed to read guardrail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to read guardrail: status %s", resp.Status)
	}

	var result struct {
		GuardrailName string                 `json:"guardrail_name"`
		LiteLLMParams map[string]interface{} `json:"litellm_params"`
		GuardrailInfo map[string]interface{} `json:"guardrail_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode guardrail: %w", err)
	}
	if err := d.Set("guardrail_name", result.GuardrailName); err != nil {
		return err
	}
	info := result.GuardrailInfo
	if info == nil {
		info = map[string]interface{}{}
	}
	encodedInfo, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to encode guardrail_info: %w", err)
	}
	if err := d.Set("guardrail_info", string(encodedInfo)); err != nil {
		return err
	}
	// LiteLLM masks secrets on reads. Preserve configured params; imports still
	// receive the masked definition so users can replace it in configuration.
	if _, configured := d.GetOk("litellm_params"); !configured {
		encodedParams, err := json.Marshal(result.LiteLLMParams)
		if err != nil {
			return fmt.Errorf("failed to encode litellm_params: %w", err)
		}
		if err := d.Set("litellm_params", string(encodedParams)); err != nil {
			return err
		}
	}
	return nil
}

func resourceLiteLLMGuardrailUpdate(d *schema.ResourceData, m interface{}) error {
	payload, err := guardrailPayload(d)
	if err != nil {
		return err
	}
	if _, err := sendGuardrailRequest(m.(*Client), http.MethodPut, "/guardrails/"+d.Id(), payload); err != nil {
		return fmt.Errorf("failed to update guardrail: %w", err)
	}
	return resourceLiteLLMGuardrailRead(d, m)
}

func resourceLiteLLMGuardrailDelete(d *schema.ResourceData, m interface{}) error {
	if _, err := sendGuardrailRequest(m.(*Client), http.MethodDelete, "/guardrails/"+d.Id(), nil); err != nil {
		return fmt.Errorf("failed to delete guardrail: %w", err)
	}
	d.SetId("")
	return nil
}
