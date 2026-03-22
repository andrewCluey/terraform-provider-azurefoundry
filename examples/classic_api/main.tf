terraform {
  required_providers {
    azurefoundry = {
      source = "andrewCluey/azurefoundry"
      version = "0.1.1"
    }
  }
}

provider "azurefoundry" {
  project_endpoint = "https://foundry-project.resource.services.ai.azure.com/api/projects/foundry-project"
  use_azure_cli    = true
}



# ── Example 1: Simple agent with code interpreter ─────────────────────────────
/*
resource "azurefoundry_agent" "simple" {
  model        = "gpt-4o"
  name         = "simple-assistant"
  instructions = "You are a helpful assistant."

  tools {
    type = "code_interpreter"
  }
}
*/

# ── Example 2: Agent with file search ─────────────────────────────────────────
# Step 1: Upload a file
resource "azurefoundry_file" "knowledge" {
  source = "./SKILL-refactor-module.md"   # path to a local file
}

# Step 2: Create a vector store and index the file
resource "azurefoundry_vector_store" "knowledge" {
  name     = "knowledge-base"
  file_ids = [azurefoundry_file.knowledge.id]

  expiry_anchor = "last_active_at"
  expiry_days   = 7

  metadata = {
    environment = "dev"
  }
}

# Step 3: Create an agent with file_search pointing at the vector store
resource "azurefoundry_agent" "researcher" {
  model        = "4.1-nano"
  name         = "dev-41"
  instructions = "You are a dev assistant. Use the file search tool to answer questions from the knowledge base."
  temperature = 0.5
  tools {
    type = "file_search"
  }

  file_search_vector_store_ids = [azurefoundry_vector_store.knowledge.id]
}

# ── Outputs ───────────────────────────────────────────────────────────────────

#output "simple_agent_id" {
#  value = azurefoundry_agent.simple.id
#}

output "researcher_agent_id" {
  value = azurefoundry_agent.researcher.id
}

output "file_id" {
  value = azurefoundry_file.knowledge.id
}

output "vector_store_id" {
  value = azurefoundry_vector_store.knowledge.id
}
