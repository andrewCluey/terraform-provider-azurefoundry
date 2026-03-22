terraform {
  required_providers {
    azurefoundry = {
      source = "local/azurefoundry"
    }
  }
}

provider "azurefoundry" {
  project_endpoint = "https://foundry-project.resource.services.ai.azure.com/api/projects/foundry-project"
  use_azure_cli    = true
}




# ── Example 2: Agent with file search ─────────────────────────────────────────
# Step 1: Upload a file
resource "azurefoundry_file_v2" "tf_refactor" {
  source = "./SKILL-refactor-module.md"   # path to a local file
}

resource "azurefoundry_file_v2" "tf_style" {
  source = "./SKILL-terraform-style-guide.md"
}

# Step 2: Create a vector store and index the file
resource "azurefoundry_vector_store_v2" "knowledge" {
  name     = "tf-skills"
  file_ids = [
    azurefoundry_file_v2.tf_refactor.id,
    azurefoundry_file_v2.tf_style.id
  ]

  expiry_anchor = "last_active_at"
  expiry_days   = 7

  metadata = {
    environment = "dev"
  }
}

# Step 3: Create an agent with file_search pointing at the vector store
resource "azurefoundry_agent_v2" "researcher" {
  model        = "gpt-5.4-nano"
  name         = "t-devf"
  kind         = "prompt"
  instructions = "You are a Terraform specialist. Use the file search tool to provide better analysis of terraform code reviews & refactor."
  tools {
    type             = "file_search"
    vector_store_ids = [azurefoundry_vector_store_v2.knowledge.id]
    max_num_results  = 20
  }
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "vector" {
  value = azurefoundry_vector_store_v2.knowledge
}

output "agent" {
  value = azurefoundry_agent_v2.researcher
}