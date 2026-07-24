package contentmoderation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaiduAuthURL     = "https://aip.baidubce.com/oauth/2.0/token"
	defaultBaiduTextURL     = "https://aip.baidubce.com/rest/2.0/solution/v1/text_censor/v2/user_defined"
	defaultBaiduImageURL    = "https://aip.baidubce.com/rest/2.0/solution/v1/img_censor/v2/user_defined"
	baiduMaxTextBytes       = 20000
	baiduMaxImageBytes      = 7 << 20
	baiduResponseMaxBytes   = 1 << 20
	baiduTextModel          = "baidu"
	baiduImageModel         = "baidu_image"
	baiduDisabledModel      = "baidu_disabled"
	baiduImageDisabledModel = "baidu_image_disabled"
	baiduMaxAttempts        = 3
)

// BaiduConfig contains server-only credentials for Baidu Content Censor.
// APIKey and SecretKey must never be exposed to the browser or written to
// logs. AppID is retained for operator bookkeeping; Baidu's OAuth endpoint
// requires APIKey (client_id) and SecretKey (client_secret), not AppID.
type BaiduConfig struct {
	Enabled         bool
	ImageEnabled    bool
	APIKey          string
	SecretKey       string
	AppID           string
	AuthURL         string
	Endpoint        string
	ImageEndpoint   string
	StrategyID      string
	ImageStrategyID string
	Timeout         time.Duration
	HTTPClient      *http.Client
}

// BaiduClient implements the strict text-censor API. A suspicious or failed
// response is never treated as allowed.
type BaiduClient struct {
	enabled         bool
	imageEnabled    bool
	apiKey          string
	secretKey       string
	appID           string
	authURL         string
	endpoint        string
	imageEndpoint   string
	strategyID      string
	imageStrategyID string
	timeout         time.Duration
	httpClient      *http.Client

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

type baiduTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	Error       string `json:"error"`
}

type baiduCensorResponse struct {
	Conclusion     string `json:"conclusion"`
	ConclusionType int    `json:"conclusionType"`
	ErrorCode      int    `json:"error_code"`
	ErrorMessage   string `json:"error_msg"`
	Data           []struct {
		ConclusionType int `json:"conclusionType"`
	} `json:"data"`
}

// NewBaidu creates a Baidu client. Disabled clients deliberately accept
// empty credentials so deployments can ship the integration before an AK/SK
// pair is configured.
func NewBaidu(config BaiduConfig) (*BaiduClient, error) {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	client := &BaiduClient{
		enabled:         config.Enabled,
		imageEnabled:    config.ImageEnabled,
		apiKey:          strings.TrimSpace(config.APIKey),
		secretKey:       strings.TrimSpace(config.SecretKey),
		appID:           strings.TrimSpace(config.AppID),
		strategyID:      strings.TrimSpace(config.StrategyID),
		imageStrategyID: strings.TrimSpace(config.ImageStrategyID),
		timeout:         timeout,
		httpClient:      httpClient,
	}
	if !client.enabled && !client.imageEnabled {
		return client, nil
	}
	if client.apiKey == "" || client.secretKey == "" {
		return nil, errors.New("baidu content moderation API key and secret key are required when enabled")
	}
	var err error
	client.authURL, err = normalizeBaiduURL(config.AuthURL, defaultBaiduAuthURL, "baidu auth URL")
	if err != nil {
		return nil, err
	}
	if client.enabled {
		client.endpoint, err = normalizeBaiduURL(config.Endpoint, defaultBaiduTextURL, "baidu content moderation URL")
		if err != nil {
			return nil, err
		}
	}
	if client.imageEnabled {
		client.imageEndpoint, err = normalizeBaiduURL(config.ImageEndpoint, defaultBaiduImageURL, "baidu image moderation URL")
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

// BaiduFromEnv loads the optional second moderation provider. It is separate
// from FromEnv so existing Grok-compatible configuration remains compatible.
func BaiduFromEnv() (*BaiduClient, error) {
	enabled, err := parseBoolEnv("ROBLOX_BAIDU_CONTENT_MODERATION_ENABLED")
	if err != nil {
		return nil, err
	}
	imageEnabled, imageEnabledSet, err := parseOptionalBoolEnv("ROBLOX_BAIDU_IMAGE_MODERATION_ENABLED")
	if err != nil {
		return nil, err
	}
	if !imageEnabledSet {
		imageEnabled = enabled
	}
	timeout, err := moderationTimeoutFromEnv()
	if err != nil {
		return nil, err
	}
	return NewBaidu(BaiduConfig{
		Enabled:         enabled,
		ImageEnabled:    imageEnabled,
		APIKey:          os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_API_KEY"),
		SecretKey:       os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_SECRET_KEY"),
		AppID:           os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_APP_ID"),
		AuthURL:         os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_AUTH_URL"),
		Endpoint:        os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_URL"),
		ImageEndpoint:   os.Getenv("ROBLOX_BAIDU_IMAGE_MODERATION_URL"),
		StrategyID:      os.Getenv("ROBLOX_BAIDU_CONTENT_MODERATION_STRATEGY_ID"),
		ImageStrategyID: os.Getenv("ROBLOX_BAIDU_IMAGE_MODERATION_STRATEGY_ID"),
		Timeout:         timeout,
	})
}

func parseBoolEnv(key string) (bool, error) {
	value, _, err := parseOptionalBoolEnv(key)
	return value, err
}

func parseOptionalBoolEnv(key string) (bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, fmt.Errorf("%s must be true or false", key)
	}
	return value, true, nil
}

func (c *BaiduClient) Enabled() bool { return c != nil && c.enabled }

func (c *BaiduClient) ImageEnabled() bool { return c != nil && c.imageEnabled }

func (c *BaiduClient) Review(ctx context.Context, _ string, content string) (Result, error) {
	if c == nil || !c.enabled {
		return Result{Model: baiduDisabledModel}, nil
	}
	if strings.TrimSpace(content) == "" {
		return Result{Model: baiduTextModel}, nil
	}
	if len([]byte(content)) > baiduMaxTextBytes {
		return Result{Model: baiduTextModel}, fmt.Errorf("%w: content is too long for baidu", ErrUnavailable)
	}
	if reason := fixedPolicyReason(content); reason != "" {
		return Result{Violation: true, Reason: reason, Model: "local_policy"}, nil
	}
	form := url.Values{"text": {content}}
	if c.strategyID != "" {
		form.Set("strategyId", c.strategyID)
	}
	return c.reviewForm(ctx, c.endpoint, form, baiduTextModel)
}

func (c *BaiduClient) ReviewImage(ctx context.Context, _ string, image []byte) (Result, error) {
	if c == nil || !c.imageEnabled {
		return Result{Model: baiduImageDisabledModel}, nil
	}
	if len(image) == 0 {
		return Result{Model: baiduImageModel}, fmt.Errorf("%w: image is empty", ErrUnavailable)
	}
	if len(image) > baiduMaxImageBytes {
		return Result{Model: baiduImageModel}, fmt.Errorf("%w: image is too large for baidu", ErrUnavailable)
	}
	form := url.Values{"image": {base64.StdEncoding.EncodeToString(image)}}
	if c.imageStrategyID != "" {
		form.Set("strategyId", c.imageStrategyID)
	}
	return c.reviewForm(ctx, c.imageEndpoint, form, baiduImageModel)
}

func (c *BaiduClient) reviewForm(ctx context.Context, endpointURL string, form url.Values, model string) (Result, error) {
	encodedForm := form.Encode()
	var lastErr error
	for attempt := 0; attempt < baiduMaxAttempts; attempt++ {
		token, err := c.getAccessToken(ctx)
		if err != nil {
			return Result{Model: model}, err
		}
		endpoint := endpointURL + "?access_token=" + url.QueryEscape(token)
		requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, strings.NewReader(encodedForm))
		if err != nil {
			cancel()
			return Result{Model: model}, fmt.Errorf("%w: baidu request creation failed", ErrUnavailable)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("%w: baidu request failed", ErrUnavailable)
			if ctx.Err() != nil || attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return Result{Model: model}, lastErr
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, baiduResponseMaxBytes))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = fmt.Errorf("%w: baidu response read failed", ErrUnavailable)
			if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return Result{Model: model}, lastErr
			}
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized {
			c.clearAccessToken()
			lastErr = fmt.Errorf("%w: baidu token was rejected", ErrUnavailable)
			if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return Result{Model: model}, lastErr
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%w: baidu provider returned status %d", ErrUnavailable, resp.StatusCode)
			if !baiduTransientStatus(resp.StatusCode) || attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return Result{Model: model}, lastErr
			}
			continue
		}
		if baiduTokenRejected(body) {
			c.clearAccessToken()
			lastErr = fmt.Errorf("%w: baidu token was rejected", ErrUnavailable)
			if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return Result{Model: model}, lastErr
			}
			continue
		}
		result, parseErr := parseBaiduDecisionForModel(body, model)
		if parseErr == nil {
			return result, nil
		}
		lastErr = parseErr
		if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
			return result, lastErr
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: baidu moderation failed", ErrUnavailable)
	}
	return Result{Model: model}, lastErr
}

func (c *BaiduClient) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.apiKey},
		"client_secret": {c.secretKey},
	}
	encodedForm := form.Encode()
	var lastErr error
	for attempt := 0; attempt < baiduMaxAttempts; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.authURL, strings.NewReader(encodedForm))
		if err != nil {
			cancel()
			return "", fmt.Errorf("%w: baidu token request creation failed", ErrUnavailable)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("%w: baidu token request failed", ErrUnavailable)
			if ctx.Err() != nil || attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return "", lastErr
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = fmt.Errorf("%w: baidu token response read failed", ErrUnavailable)
			if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return "", lastErr
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%w: baidu token provider returned status %d", ErrUnavailable, resp.StatusCode)
			if !baiduTransientStatus(resp.StatusCode) || attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return "", lastErr
			}
			continue
		}
		var tokenResponse baiduTokenResponse
		if err := json.Unmarshal(body, &tokenResponse); err != nil || tokenResponse.AccessToken == "" {
			lastErr = fmt.Errorf("%w: baidu token response is invalid", ErrUnavailable)
			if attempt == baiduMaxAttempts-1 || !waitBaiduRetry(ctx, attempt) {
				return "", lastErr
			}
			continue
		}
		expiresIn := tokenResponse.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 300
		}
		c.accessToken = tokenResponse.AccessToken
		c.tokenExpiry = time.Now().Add(time.Duration(expiresIn)*time.Second - time.Minute)
		if !c.tokenExpiry.After(time.Now()) {
			c.tokenExpiry = time.Now().Add(30 * time.Second)
		}
		return c.accessToken, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: baidu token request failed", ErrUnavailable)
	}
	return "", lastErr
}

func baiduTransientStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func waitBaiduRetry(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt+1) * 250 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (c *BaiduClient) clearAccessToken() {
	c.tokenMu.Lock()
	c.accessToken = ""
	c.tokenExpiry = time.Time{}
	c.tokenMu.Unlock()
}

func baiduTokenRejected(body []byte) bool {
	var response struct {
		ErrorCode int `json:"error_code"`
	}
	return json.Unmarshal(body, &response) == nil && (response.ErrorCode == 110 || response.ErrorCode == 111)
}

func parseBaiduDecision(body []byte) (Result, error) {
	return parseBaiduDecisionForModel(body, baiduTextModel)
}

func parseBaiduDecisionForModel(body []byte, model string) (Result, error) {
	var response baiduCensorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return Result{Model: model}, fmt.Errorf("%w: baidu response is invalid", ErrUnavailable)
	}
	if response.ErrorCode != 0 {
		return Result{Model: model}, fmt.Errorf("%w: baidu response reported an error", ErrUnavailable)
	}
	conclusionType := response.ConclusionType
	if conclusionType == 0 && len(response.Data) > 0 {
		conclusionType = response.Data[0].ConclusionType
	}
	switch conclusionType {
	case 1:
		return Result{Model: model}, nil
	case 2:
		return Result{Violation: true, Reason: "百度审核不通过", Model: model}, nil
	case 3:
		return Result{Violation: true, Reason: "百度审核疑似违规", Model: model}, nil
	case 4:
		return Result{Model: model}, fmt.Errorf("%w: baidu moderation failed", ErrUnavailable)
	default:
		return Result{Model: model}, fmt.Errorf("%w: baidu conclusion is missing", ErrUnavailable)
	}
}

func normalizeBaiduURL(raw, fallback, label string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s is invalid", label)
	}
	if parsed.Scheme != "https" && !isLocalHTTP(parsed) {
		return "", fmt.Errorf("%s must use HTTPS", label)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
