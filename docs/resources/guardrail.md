---
page_title: "litellm_guardrail Resource - terraform-provider-litellm"
subcategory: ""
description: |-
  Manages a DB-backed LiteLLM guardrail definition.
---

# litellm_guardrail (Resource)

Manages a DB-backed LiteLLM guardrail definition. The provider credential must have proxy-admin access.

`litellm_params` accepts JSON so provider-specific nested options remain available without waiting for a provider release. It is sensitive and redacted from Terraform output, but still stored in Terraform state; protect the state backend accordingly.

## Example Usage

```terraform
resource "litellm_guardrail" "request_filter" {
  guardrail_name = "request-filter"

  litellm_params = jsonencode({
    guardrail = "generic_guardrail_api"
    mode      = ["pre_call", "post_call"]
    api_base  = "https://guardrail.example.com"
    api_key   = var.guardrail_api_key
    additional_provider_specific_params = {
      threshold = 0.8
    }
  })

  guardrail_info = jsonencode({
    description = "Protect requests and responses"
  })
}
```

## Schema

- `guardrail_name` (Required) — unique guardrail name.
- `litellm_params` (Required, Sensitive) — JSON object passed to LiteLLM as `litellm_params`.
- `guardrail_info` (Optional) — JSON metadata object; defaults to `{}`.

## Import

Import by LiteLLM guardrail ID:

```shell
terraform import litellm_guardrail.request_filter 123e4567-e89b-12d3-a456-426614174000
```

LiteLLM masks secrets on reads. Imported `litellm_params` therefore contains masked values until real credentials are supplied in configuration.
