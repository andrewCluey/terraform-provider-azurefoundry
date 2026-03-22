# terraform-provider-azurefoundry

A Terraform provider for managing Azure AI Foundry Agents.

## Running locally

### 1. Sort out dependencies

```bash
go mod tidy
```

### 2. Build and install the provider binary

```bash
go install .
```

This compiles the provider and puts the binary in your Go bin directory.

### 3. Tell Terraform to use your local binary

Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "local/azurefoundry" = "/Users/andrew/go/bin"
  }
  direct {}
}
```

Where `/Users/andrew/go/bin` is the output of `go env GOBIN` (or `go env GOPATH` + `/bin` if GOBIN is empty).

### 4. Use the provider

from root of the provider project, run `go install ./...`

From the `examples/` folder:

```bash
cd examples
terraform plan
terraform apply
```

Note: because of the `dev_overrides` in step 3, you do **not** need to run `terraform init`.

## Authentication

The provider tries authentication methods in this order:

1. **API Key** — set `api_key` in the provider block or `AZURE_AI_FOUNDRY_API_KEY` env var
2. **Service Principal** — set `tenant_id`, `client_id`, `client_secret` (or `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`)
3. **Azure CLI** — set `use_azure_cli = true` in the provider block (run `az login` first)
4. **Default Azure credential chain** — managed identity, workload identity, etc.

## Provider configuration

```hcl
provider "azurefoundry" {
  project_endpoint = "https://<resource>.services.ai.azure.com/api/projects/<project>"

  # Pick one auth method:
  use_azure_cli = true

  # or service principal:
  # tenant_id     = "..."
  # client_id     = "..."
  # client_secret = "..."

  # or API key:
  # api_key = "..."
}
```

## Resources

### azurefoundry_agent

```hcl
terraform {
  required_providers {
    azurefoundry = {
      source = "local/azurefoundry"
    }
  }
}

resource "azurefoundry_agent" "example" {
  model        = "gpt-4o"
  name         = "my-agent"
  description  = "A helpful assistant"
  instructions = "You are a helpful assistant."
  temperature  = 0.7

  tools {
    type = "code_interpreter"
  }

  metadata = {
    environment = "production"
  }
}
```

#### Arguments

| Name | Required | Description |
|------|----------|-------------|
| `model` | Yes | Model deployment name e.g. `gpt-4o` |
| `name` | No | Display name |
| `description` | No | Short description |
| `instructions` | No | System prompt |
| `temperature` | No | 0–2, controls randomness |
| `top_p` | No | 0–1, nucleus sampling |
| `metadata` | No | Up to 16 key/value pairs |
| `tools` | No | Blocks, each with a `type` |
| `code_interpreter_file_ids` | No | File IDs for the code interpreter tool |

#### Tool types

- `code_interpreter`
- `file_search`
- `bing_grounding`
- `azure_ai_search`
- `azure_function`

#### Attributes (read-only)

| Name | Description |
|------|-------------|
| `id` | Agent ID assigned by Foundry |
| `created_at` | Unix timestamp of creation |

#### Import

```bash
terraform import azurefoundry_agent.example <agent_id>
```
