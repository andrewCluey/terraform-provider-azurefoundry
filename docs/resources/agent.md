---
page_title: "azurefoundry_agent Resource - azurefoundry"
description: |-
  Manages an Azure AI Foundry Agent.
---

# azurefoundry_agent

Manages an **Azure AI Foundry Agent**. An agent pairs a deployed language model with instructions, tools, and optional knowledge resources.

## Example Usage

### Simple agent

```hcl
resource "azurefoundry_agent" "example" {
  model        = "gpt-4o"
  name         = "my-assistant"
  instructions = "You are a helpful assistant."
}
```

### Agent with code interpreter

```hcl
resource "azurefoundry_agent" "analyst" {
  model        = "gpt-4o"
  name         = "data-analyst"
  instructions = "You are a data analyst. Write clear, well-commented Python code."

  tools {
    type = "code_interpreter"
  }

  temperature = 0.2
}
```

### Agent with file search

```hcl
resource "azurefoundry_file" "knowledge" {
  source = "./knowledge.pdf"
}

resource "azurefoundry_vector_store" "knowledge" {
  name     = "knowledge-base"
  file_ids = [azurefoundry_file.knowledge.id]
}

resource "azurefoundry_agent" "researcher" {
  model        = "gpt-4o" # the model deployment (cognitive deployment) must already exist in the foundry project
  name         = "research-assistant"
  instructions = "Answer questions using the knowledge base."

  tools {
    type = "file_search"
  }

  file_search_vector_store_ids = [azurefoundry_vector_store.knowledge.id]
}
```

## Argument Reference

### Required

- `model` (String) - The model deployment name (e.g. `gpt-4o`, `gpt-4o-mini`). This deployment must already exist in the Foundry project before you deploy.

### Optional

- `name` (String) - Display name for the agent (≤ 256 characters).
- `description` (String) - Short description (≤ 512 characters).
- `instructions` (String) - System prompt for the agent (≤ 256,000 characters).
- `temperature` (Number) - Sampling temperature between `0` and `2`.
- `top_p` (Number) - Nucleus sampling parameter between `0` and `1`.
- `metadata` (Map of String) - Up to 16 key/value pairs.
- `code_interpreter_file_ids` (List of String) - File IDs for the code interpreter tool. Maximum 20.
- `file_search_vector_store_ids` (List of String) - Vector store IDs for the file search tool. Maximum 1.

### `tools` Block

- `type` (String, Required) - Tool type. One of: `code_interpreter`, `file_search`, `bing_grounding`, `azure_ai_search`, `azure_function`.

## Attributes Reference

- `id` (String) - The agent ID assigned by the Foundry service.
- `created_at` (Number) - Unix timestamp of creation.

## Import

```shell
terraform import azurefoundry_agent.example <agent_id>
```
