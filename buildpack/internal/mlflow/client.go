// Package mlflow provides MLflow Model Registry integration.
package mlflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

const (
	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Minute
)

// ModelVersion represents a version of a registered model.
type ModelVersion struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	ArtifactURI string `json:"artifact_uri"`
	Status      string `json:"status"`
}

// modelVersionResponse is the API response for model version.
type modelVersionResponse struct {
	ModelVersion ModelVersion `json:"model_version"`
}

// searchModelVersionsResponse is the API response for searching model versions.
type searchModelVersionsResponse struct {
	ModelVersions []ModelVersion `json:"model_versions"`
}

// Client provides access to MLflow Model Registry.
type Client struct {
	httpClient  *http.Client
	trackingURI string
	username    string
	password    string
}

// ClientOption is a functional option for configuring the client.
type ClientOption func(*Client)

// WithCredentials sets authentication credentials.
func WithCredentials(username, password string) ClientOption {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

// WithTimeout sets the HTTP timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new MLflow client.
func NewClient(trackingURI string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		trackingURI: trackingURI,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GetModelVersion retrieves a specific version of a model.
func (c *Client) GetModelVersion(ctx context.Context, name, version string) (*ModelVersion, error) {
	endpoint := fmt.Sprintf("/api/2.0/mlflow/registered-models/get-version")
	params := url.Values{}
	params.Set("name", name)
	params.Set("version", version)

	resp, err := c.doRequest(ctx, "GET", endpoint, params, nil)
	if err != nil {
		return nil, fmt.Errorf("getting model version: %w", err)
	}
	defer resp.Body.Close()

	var result modelVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result.ModelVersion, nil
}

// GetLatestModelVersion retrieves the latest version of a model.
func (c *Client) GetLatestModelVersion(ctx context.Context, name string) (*ModelVersion, error) {
	endpoint := "/api/2.0/mlflow/registered-models/search-versions"
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("name='%s'", name))

	resp, err := c.doRequest(ctx, "GET", endpoint, params, nil)
	if err != nil {
		return nil, fmt.Errorf("searching model versions: %w", err)
	}
	defer resp.Body.Close()

	var result searchModelVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.ModelVersions) == 0 {
		return nil, fmt.Errorf("no versions found for model %s", name)
	}

	// Return the first (latest) version
	return &result.ModelVersions[0], nil
}

// GetModelVersionByStage retrieves a model version by stage.
func (c *Client) GetModelVersionByStage(ctx context.Context, name, stage string) (*ModelVersion, error) {
	endpoint := "/api/2.0/mlflow/registered-models/search-versions"
	params := url.Values{}
	params.Set("filter", fmt.Sprintf("name='%s' AND current_stage='%s'", name, stage))

	resp, err := c.doRequest(ctx, "GET", endpoint, params, nil)
	if err != nil {
		return nil, fmt.Errorf("searching model versions by stage: %w", err)
	}
	defer resp.Body.Close()

	var result searchModelVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.ModelVersions) == 0 {
		return nil, fmt.Errorf("no version found for model %s in stage %s", name, stage)
	}

	return &result.ModelVersions[0], nil
}

// ResolveModelVersion resolves the model version from name and version/stage.
func (c *Client) ResolveModelVersion(ctx context.Context, name, versionOrStage string) (*ModelVersion, error) {
	if versionOrStage == "" || versionOrStage == "latest" {
		return c.GetLatestModelVersion(ctx, name)
	}

	// Check if it's a numeric version
	if isNumericVersion(versionOrStage) {
		return c.GetModelVersion(ctx, name, versionOrStage)
	}

	// Treat as stage name
	return c.GetModelVersionByStage(ctx, name, versionOrStage)
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, params url.Values, body io.Reader) (*http.Response, error) {
	baseURL, err := url.Parse(c.trackingURI)
	if err != nil {
		return nil, fmt.Errorf("parsing tracking URI: %w", err)
	}

	reqURL := baseURL.ResolveReference(&url.URL{Path: endpoint})
	if params != nil {
		reqURL.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	return resp, nil
}

func isNumericVersion(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return len(s) > 0
}

// ParseArtifactURI parses an artifact URI and returns the scheme and path.
func ParseArtifactURI(uri string) (scheme, path string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parsing artifact URI: %w", err)
	}
	return u.Scheme, u.Path, nil
}

// IsS3URI checks if the URI is an S3 URI.
func IsS3URI(uri string) bool {
	scheme, _, err := ParseArtifactURI(uri)
	if err != nil {
		return false
	}
	return scheme == "s3"
}

// IsLocalURI checks if the URI is a local file path.
func IsLocalURI(uri string) bool {
	scheme, _, err := ParseArtifactURI(uri)
	if err != nil {
		return false
	}
	return scheme == "" || scheme == "file"
}

// JoinArtifactPath joins a base artifact URI with a subpath.
func JoinArtifactPath(baseURI, subpath string) string {
	u, err := url.Parse(baseURI)
	if err != nil {
		return path.Join(baseURI, subpath)
	}
	u.Path = path.Join(u.Path, subpath)
	return u.String()
}
