// Copyright (c) Your Org
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

const APIVersion = "2025-05-01"

type AuthMode int

const (
	AuthModeAzureCredential AuthMode = iota
	AuthModeAPIKey
)

// FoundryClient is an authenticated client for the Azure AI Foundry Agent Service.
type FoundryClient struct {
	ProjectEndpoint string
	authMode        AuthMode
	credential      azcore.TokenCredential
	apiKey          string
	httpClient      *http.Client
}

func NewFoundryClientWithCredential(projectEndpoint string, credential azcore.TokenCredential) *FoundryClient {
	return &FoundryClient{
		ProjectEndpoint: projectEndpoint,
		authMode:        AuthModeAzureCredential,
		credential:      credential,
		httpClient:      &http.Client{Timeout: 60 * time.Second},
	}
}

func NewFoundryClientWithAPIKey(projectEndpoint string, apiKey string) *FoundryClient {
	return &FoundryClient{
		ProjectEndpoint: projectEndpoint,
		authMode:        AuthModeAPIKey,
		apiKey:          apiKey,
		httpClient:      &http.Client{Timeout: 60 * time.Second},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent model types
// ─────────────────────────────────────────────────────────────────────────────

type ToolDefinition struct {
	Type string `json:"type"`
}

type CodeInterpreterResources struct {
	FileIDs []string `json:"file_ids,omitempty"`
}

type FileSearchResources struct {
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
}

type ToolResources struct {
	CodeInterpreter *CodeInterpreterResources `json:"code_interpreter,omitempty"`
	FileSearch      *FileSearchResources      `json:"file_search,omitempty"`
}

type CreateAgentRequest struct {
	Model         string            `json:"model"`
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	Instructions  string            `json:"instructions,omitempty"`
	Tools         []interface{}     `json:"tools,omitempty"`
	ToolResources *ToolResources    `json:"tool_resources,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type AgentResponse struct {
	ID            string            `json:"id"`
	Object        string            `json:"object"`
	CreatedAt     int64             `json:"created_at"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Model         string            `json:"model"`
	Instructions  string            `json:"instructions"`
	Tools         []interface{}     `json:"tools"`
	ToolResources *ToolResources    `json:"tool_resources"`
	Temperature   *float64          `json:"temperature"`
	TopP          *float64          `json:"top_p"`
	Metadata      map[string]string `json:"metadata"`
}

type UpdateAgentRequest struct {
	Model         string            `json:"model,omitempty"`
	Name          string            `json:"name,omitempty"`
	Description   string            `json:"description,omitempty"`
	Instructions  string            `json:"instructions,omitempty"`
	Tools         []interface{}     `json:"tools,omitempty"`
	ToolResources *ToolResources    `json:"tool_resources,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type DeleteAgentResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// v2 - new API
type AgentDefinitionV2 struct {
    Kind         string `json:"kind"`
    Model        string `json:"model,omitempty"`
    Instructions string `json:"instructions,omitempty"`
	Tools        []interface{} `json:"tools,omitempty"`
}

type AgentVersionV2 struct {
    Object      string            `json:"object"`
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Version     string            `json:"version"`
    Description string            `json:"description"`
    CreatedAt   int64             `json:"created_at"`
    Metadata    map[string]string `json:"metadata"`
    Definition  AgentDefinitionV2 `json:"definition"`
}

type AgentResponseV2 struct {
    Object   string `json:"object"`
    ID       string `json:"id"`
    Name     string `json:"name"`
    Versions struct {
        Latest AgentVersionV2 `json:"latest"`
    } `json:"versions"`
}

type CreateAgentV2Request struct {
    Name        string            `json:"name"`
    Description string            `json:"description,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    Definition  AgentDefinitionV2 `json:"definition"`
}

type UpdateAgentV2Request struct {
    Description string            `json:"description,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    Definition  AgentDefinitionV2 `json:"definition"`
}

type DeleteAgentV2Response struct {
    Object  string `json:"object"`
    Name    string `json:"name"`
    Deleted bool   `json:"deleted"`
}

type FileSearchToolV2 struct {
    Type           string   `json:"type"`
    VectorStoreIDs []string `json:"vector_store_ids,omitempty"`
    MaxNumResults  int      `json:"max_num_results,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// File model types
// ─────────────────────────────────────────────────────────────────────────────

type FilePurpose string

const FilePurposeAssistants FilePurpose = "assistants"

type FileResponse struct {
	ID        string      `json:"id"`
	Object    string      `json:"object"`
	Bytes     int64       `json:"bytes"`
	CreatedAt int64       `json:"created_at"`
	Filename  string      `json:"filename"`
	Purpose   FilePurpose `json:"purpose"`
}

type DeleteFileResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Vector store model types
// ─────────────────────────────────────────────────────────────────────────────

type VectorStoreStatus string

const (
	VectorStoreStatusInProgress VectorStoreStatus = "in_progress"
	VectorStoreStatusCompleted  VectorStoreStatus = "completed"
	VectorStoreStatusExpired    VectorStoreStatus = "expired"
)

type VectorStoreExpirationPolicy struct {
	Anchor string `json:"anchor,omitempty"`
	Days   int64  `json:"days,omitempty"`
}

type VectorStoreFileCounts struct {
	InProgress int64 `json:"in_progress"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	Cancelled  int64 `json:"cancelled"`
	Total      int64 `json:"total"`
}

type VectorStoreResponse struct {
	ID           string                       `json:"id"`
	Object       string                       `json:"object"`
	CreatedAt    int64                        `json:"created_at"`
	Name         string                       `json:"name"`
	UsageBytes   int64                        `json:"usage_bytes"`
	FileCounts   VectorStoreFileCounts        `json:"file_counts"`
	Status       VectorStoreStatus            `json:"status"`
	ExpiresAfter *VectorStoreExpirationPolicy `json:"expires_after,omitempty"`
	ExpiresAt    *int64                       `json:"expires_at,omitempty"`
	LastActiveAt int64                        `json:"last_active_at"`
	Metadata     map[string]string            `json:"metadata"`
}

type CreateVectorStoreRequest struct {
	Name         string                       `json:"name,omitempty"`
	FileIDs      []string                     `json:"file_ids,omitempty"`
	ExpiresAfter *VectorStoreExpirationPolicy `json:"expires_after,omitempty"`
	Metadata     map[string]string            `json:"metadata,omitempty"`
}

type UpdateVectorStoreRequest struct {
	Name         string                       `json:"name,omitempty"`
	ExpiresAfter *VectorStoreExpirationPolicy `json:"expires_after,omitempty"`
	Metadata     map[string]string            `json:"metadata,omitempty"`
}

type DeleteVectorStoreResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent CRUD
// ─────────────────────────────────────────────────────────────────────────────


// classic foundry hub API
func (c *FoundryClient) CreateAgent(ctx context.Context, req CreateAgentRequest) (*AgentResponse, error) {
	url := fmt.Sprintf("%s/assistants?api-version=%s", c.ProjectEndpoint, APIVersion)
	return c.doAgentRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) GetAgent(ctx context.Context, agentID string) (*AgentResponse, error) {
	url := fmt.Sprintf("%s/assistants/%s?api-version=%s", c.ProjectEndpoint, agentID, APIVersion)
	return c.doAgentRequest(ctx, http.MethodGet, url, nil)
}

func (c *FoundryClient) UpdateAgent(ctx context.Context, agentID string, req UpdateAgentRequest) (*AgentResponse, error) {
	url := fmt.Sprintf("%s/assistants/%s?api-version=%s", c.ProjectEndpoint, agentID, APIVersion)
	return c.doAgentRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) DeleteAgent(ctx context.Context, agentID string) (*DeleteAgentResponse, error) {
	url := fmt.Sprintf("%s/assistants/%s?api-version=%s", c.ProjectEndpoint, agentID, APIVersion)
	httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("delete agent HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result DeleteAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding delete agent response: %w", err)
	}
	return &result, nil
}

// new CRUD functions, pointing at the newer /agents Microsoft Foundry API
func (c *FoundryClient) CreateAgentV2(ctx context.Context, req CreateAgentV2Request) (*AgentResponseV2, error) {
    url := fmt.Sprintf("%s/agents?api-version=2025-11-15-preview", c.ProjectEndpoint)
    return c.doAgentV2Request(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) GetAgentV2(ctx context.Context, name string) (*AgentResponseV2, error) {
    url := fmt.Sprintf("%s/agents/%s?api-version=2025-11-15-preview", c.ProjectEndpoint, name)
    return c.doAgentV2Request(ctx, http.MethodGet, url, nil)
}

func (c *FoundryClient) UpdateAgentV2(ctx context.Context, name string, req UpdateAgentV2Request) (*AgentResponseV2, error) {
    url := fmt.Sprintf("%s/agents/%s?api-version=2025-11-15-preview", c.ProjectEndpoint, name)
    return c.doAgentV2Request(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) DeleteAgentV2(ctx context.Context, name string) (*DeleteAgentV2Response, error) {
    url := fmt.Sprintf("%s/agents/%s?api-version=2025-11-15-preview", c.ProjectEndpoint, name)
    httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
    if err != nil {
        return nil, err
    }
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("delete agent v2 HTTP error: %w", err)
    }
    defer resp.Body.Close()
    if err := checkResponseError(resp); err != nil {
        return nil, err
    }
    var result DeleteAgentV2Response
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding delete agent v2 response: %w", err)
    }
    return &result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// File CRUD
// ─────────────────────────────────────────────────────────────────────────────

// UploadFile uploads file contents with the given filename.
// Use FilePurposeAssistants for files used with file_search or code_interpreter.
func (c *FoundryClient) UploadFile(ctx context.Context, filename string, fileData []byte, purpose FilePurpose) (*FileResponse, error) {
	url := fmt.Sprintf("%s/files?api-version=%s", c.ProjectEndpoint, APIVersion)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("purpose", string(purpose)); err != nil {
		return nil, fmt.Errorf("writing purpose field: %w", err)
	}

	part, err := mw.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("writing file data: %w", err)
	}
	mw.Close()

	httpReq, err := c.newRequestRaw(ctx, http.MethodPost, url, &buf, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upload file HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponseError(resp); err != nil {
		return nil, err
	}

	var result FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding upload file response: %w", err)
	}
	return &result, nil
}

func (c *FoundryClient) GetFile(ctx context.Context, fileID string) (*FileResponse, error) {
	url := fmt.Sprintf("%s/files/%s?api-version=%s", c.ProjectEndpoint, fileID, APIVersion)
	httpReq, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get file HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding get file response: %w", err)
	}
	return &result, nil
}

func (c *FoundryClient) DeleteFile(ctx context.Context, fileID string) (*DeleteFileResponse, error) {
	url := fmt.Sprintf("%s/files/%s?api-version=%s", c.ProjectEndpoint, fileID, APIVersion)
	httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("delete file HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding delete file response: %w", err)
	}
	return &result, nil
}

// v2
func (c *FoundryClient) UploadFileV2(ctx context.Context, filename string, fileData []byte, purpose FilePurpose) (*FileResponse, error) {
    url := fmt.Sprintf("%s/files?api-version=v1", c.ProjectEndpoint)

    var buf bytes.Buffer
    mw := multipart.NewWriter(&buf)

    if err := mw.WriteField("purpose", string(purpose)); err != nil {
        return nil, fmt.Errorf("writing purpose field: %w", err)
    }

    part, err := mw.CreateFormFile("file", filepath.Base(filename))
    if err != nil {
        return nil, fmt.Errorf("creating form file: %w", err)
    }
    if _, err := part.Write(fileData); err != nil {
        return nil, fmt.Errorf("writing file data: %w", err)
    }
    mw.Close()

    httpReq, err := c.newRequestRaw(ctx, http.MethodPost, url, &buf, mw.FormDataContentType())
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("upload file v2 HTTP error: %w", err)
    }
    defer resp.Body.Close()

    if err := checkResponseError(resp); err != nil {
        return nil, err
    }

    var result FileResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding upload file v2 response: %w", err)
    }
    return &result, nil
}

func (c *FoundryClient) GetFileV2(ctx context.Context, fileID string) (*FileResponse, error) {
    url := fmt.Sprintf("%s/files/%s?api-version=v1", c.ProjectEndpoint, fileID)
    httpReq, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get file HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result FileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding get file response: %w", err)
	}
	return &result, nil
}

func (c *FoundryClient) DeleteFileV2(ctx context.Context, fileID string) (*DeleteFileResponse, error) {
    url := fmt.Sprintf("%s/files/%s?api-version=v1", c.ProjectEndpoint, fileID)
    httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("delete file HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding delete file response: %w", err)
	}
	return &result, nil
}



// ─────────────────────────────────────────────────────────────────────────────
// Vector store CRUD
// ─────────────────────────────────────────────────────────────────────────────

func (c *FoundryClient) CreateVectorStore(ctx context.Context, req CreateVectorStoreRequest) (*VectorStoreResponse, error) {
	url := fmt.Sprintf("%s/vector_stores?api-version=%s", c.ProjectEndpoint, APIVersion)
	return c.doVectorStoreRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) GetVectorStore(ctx context.Context, vectorStoreID string) (*VectorStoreResponse, error) {
	url := fmt.Sprintf("%s/vector_stores/%s?api-version=%s", c.ProjectEndpoint, vectorStoreID, APIVersion)
	return c.doVectorStoreRequest(ctx, http.MethodGet, url, nil)
}

func (c *FoundryClient) UpdateVectorStore(ctx context.Context, vectorStoreID string, req UpdateVectorStoreRequest) (*VectorStoreResponse, error) {
	url := fmt.Sprintf("%s/vector_stores/%s?api-version=%s", c.ProjectEndpoint, vectorStoreID, APIVersion)
	return c.doVectorStoreRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) DeleteVectorStore(ctx context.Context, vectorStoreID string) (*DeleteVectorStoreResponse, error) {
	url := fmt.Sprintf("%s/vector_stores/%s?api-version=%s", c.ProjectEndpoint, vectorStoreID, APIVersion)
	httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("delete vector store HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result DeleteVectorStoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding delete vector store response: %w", err)
	}
	return &result, nil
}

// WaitForVectorStore polls until the vector store status is "completed" or "expired".
func (c *FoundryClient) WaitForVectorStore(ctx context.Context, vectorStoreID string) (*VectorStoreResponse, error) {
	for {
		vs, err := c.GetVectorStore(ctx, vectorStoreID)
		if err != nil {
			return nil, err
		}
		switch vs.Status {
		case VectorStoreStatusCompleted:
			return vs, nil
		case VectorStoreStatusExpired:
			return nil, fmt.Errorf("vector store %s expired before completing ingestion", vectorStoreID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

//v2

func (c *FoundryClient) CreateVectorStoreV2(ctx context.Context, req CreateVectorStoreRequest) (*VectorStoreResponse, error) {
    url := fmt.Sprintf("%s/vector_stores?api-version=v1", c.ProjectEndpoint)
    return c.doVectorStoreRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) GetVectorStoreV2(ctx context.Context, vectorStoreID string) (*VectorStoreResponse, error) {
    url := fmt.Sprintf("%s/vector_stores/%s?api-version=v1", c.ProjectEndpoint, vectorStoreID)
    return c.doVectorStoreRequest(ctx, http.MethodGet, url, nil)
}

func (c *FoundryClient) UpdateVectorStoreV2(ctx context.Context, vectorStoreID string, req UpdateVectorStoreRequest) (*VectorStoreResponse, error) {
    url := fmt.Sprintf("%s/vector_stores/%s?api-version=v1", c.ProjectEndpoint, vectorStoreID)
    return c.doVectorStoreRequest(ctx, http.MethodPost, url, req)
}

func (c *FoundryClient) DeleteVectorStoreV2(ctx context.Context, vectorStoreID string) (*DeleteVectorStoreResponse, error) {
    url := fmt.Sprintf("%s/vector_stores/%s?api-version=v1", c.ProjectEndpoint, vectorStoreID)
	httpReq, err := c.newRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("delete vector store HTTP error: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result DeleteVectorStoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding delete vector store response: %w", err)
	}
	return &result, nil
}



// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

func (c *FoundryClient) doAgentRequest(ctx context.Context, method, url string, body interface{}) (*AgentResponse, error) {
	httpReq, err := c.newRequest(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agent API HTTP error (%s %s): %w", method, url, err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result AgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding agent response: %w", err)
	}
	return &result, nil
}

func (c *FoundryClient) doVectorStoreRequest(ctx context.Context, method, url string, body interface{}) (*VectorStoreResponse, error) {
	httpReq, err := c.newRequest(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vector store API HTTP error (%s %s): %w", method, url, err)
	}
	defer resp.Body.Close()
	if err := checkResponseError(resp); err != nil {
		return nil, err
	}
	var result VectorStoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding vector store response: %w", err)
	}
	return &result, nil
}

func (c *FoundryClient) newRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	return c.newRequestRaw(ctx, method, url, bodyReader, "application/json")
}

func (c *FoundryClient) newRequestRaw(ctx context.Context, method, url string, body io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	switch c.authMode {
	case AuthModeAPIKey:
		req.Header.Set("api-key", c.apiKey)
	case AuthModeAzureCredential:
		tokenOpts := policy.TokenRequestOptions{
			Scopes: []string{"https://ai.azure.com/.default"},
		}
		token, err := c.credential.GetToken(ctx, tokenOpts)
		if err != nil {
			return nil, fmt.Errorf("acquiring Azure token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.Token)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Azure AI Foundry API error (HTTP %d): %s", e.StatusCode, e.Body)
}

func checkResponseError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
}

// v2
func (c *FoundryClient) doAgentV2Request(ctx context.Context, method, url string, body interface{}) (*AgentResponseV2, error) {
    httpReq, err := c.newRequest(ctx, method, url, body)
    if err != nil {
        return nil, err
    }
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("agent v2 API HTTP error (%s %s): %w", method, url, err)
    }
    defer resp.Body.Close()
    if err := checkResponseError(resp); err != nil {
        return nil, err
    }
    var result AgentResponseV2
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decoding agent v2 response: %w", err)
    }
    return &result, nil
}