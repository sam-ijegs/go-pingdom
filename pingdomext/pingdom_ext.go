package pingdomext

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

const (
	defaultAuthURL = "https://my.solarwinds.cloud/v1/login"
	defaultBaseURL = "https://my.pingdom.com"
	apiURL         = "https://api.pingdom.com/api/3.1" // API URL for token-based auth
)

// Client represents a client to the Pingdom API.
type Client struct {
	JWTToken       string
	APITokenOnly   string // Renamed from APIToken to APITokenOnly
	BaseURL        *url.URL
	client         *http.Client
	Integrations   *IntegrationService
	useAPITokenOnly bool // Flag to indicate which auth method to use (renamed)
}

// ClientConfig represents a configuration for a pingdom client.
type ClientConfig struct {
	Username     string
	Password     string
	OrgID        string
	AuthURL      string
	BaseURL      string
	HTTPClient   *http.Client
	APITokenOnly string // Renamed from APIToken
}

type authPayload struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	LoginQueryParams string `json:"loginQueryParams"`
}

// ClientConfig represents a configuration for a pingdom client.
type authResult struct {
	RedirectURL string `json:"redirectUrl"`
}

// NewClientWithConfig returns a Pingdom client.
func NewClientWithConfig(config ClientConfig) (*Client, error) {
	var baseURL *url.URL
	var err error

	// Check for API token in configuration or environment
	apiTokenOnly := config.APITokenOnly
	if apiTokenOnly == "" {
		if envAPIToken, set := os.LookupEnv("PINGDOM_API_TOKEN"); set {
			apiTokenOnly = envAPIToken
		}
	}

	// Initialize client with appropriate base URL
	if config.BaseURL == "" {
		if apiTokenOnly != "" {
			config.BaseURL = apiURL // Use API URL for token auth
		} else {
			config.BaseURL = defaultBaseURL // Use default for SolarWinds auth
		}
	}

	baseURL, err = url.Parse(config.BaseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		BaseURL: baseURL,
	}

	// Set HTTP client
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	c.client = config.HTTPClient

	// If API token is provided, use token-based authentication
	if apiTokenOnly != "" {
		c.APITokenOnly = apiTokenOnly
		c.useAPITokenOnly = true
	} else {
		// Otherwise, fall back to SolarWinds authentication
		c.useAPITokenOnly = false

		// Check SolarWinds credentials from config or environment
		if config.Username == "" {
			if envUsername, set := os.LookupEnv("SOLARWINDS_USER"); set {
				config.Username = envUsername
			}
		}

		if config.Password == "" {
			if envPassword, set := os.LookupEnv("SOLARWINDS_PASSWD"); set {
				config.Password = envPassword
			}
		}

		if config.OrgID == "" {
			if envOrgID, set := os.LookupEnv("SOLARWINDS_ORG_ID"); set {
				config.OrgID = envOrgID
			}
		}

		if config.AuthURL == "" {
			config.AuthURL = defaultAuthURL
		}

		// Obtain JWT token using SolarWinds auth
		jwtToken, err := obtainToken(config)
		if err != nil {
			return nil, err
		}
		c.JWTToken = *jwtToken
	}

	c.Integrations = &IntegrationService{client: c}

	return c, nil
}

func obtainToken(config ClientConfig) (*string, error) {
	// Existing token obtainment code remains unchanged
	stateURL, err := url.Parse(config.BaseURL + "/auth/login?")
	if err != nil {
		return nil, err
	}

	stateReq, err := http.NewRequest("GET", stateURL.String(), nil)
	if err != nil {
		return nil, err
	}
	stateResp, err := config.HTTPClient.Do(stateReq)
	if err != nil {
		return nil, err
	}

	defer stateResp.Body.Close()

	location, err := stateResp.Location()
	if err != nil {
		return nil, err
	}

	sessionCookie, err := getCookie(stateResp, "pingdom_login_session_id")
	if err != nil {
		return nil, err
	}

	authPayload := authPayload{
		Email:            config.Username,
		Password:         config.Password,
		LoginQueryParams: location.Query().Encode(),
	}

	authBody, err := json.Marshal(authPayload)
	if err != nil {
		return nil, err
	}

	authReq, err := http.NewRequest("POST", config.AuthURL, bytes.NewReader(authBody))
	if err != nil {
		return nil, err
	}
	authReq.Header.Add("Content-Type", "application/json")

	authResp, err := config.HTTPClient.Do(authReq)
	if err != nil {
		return nil, err
	}
	defer authResp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(authResp.Body)
	bodyString := string(bodyBytes)

	authRespJSON := &authResult{}
	err1 := json.Unmarshal([]byte(bodyString), &authRespJSON)

	if err1 != nil {
		return nil, err1
	}

	redirectURL, err := url.Parse(authRespJSON.RedirectURL)
	if err != nil {
		return nil, err
	}
	tokenReq, err := http.NewRequest("GET", config.BaseURL+"/auth/swicus/callback?"+redirectURL.Query().Encode(), nil)
	if err != nil {
		return nil, err
	}
	tokenReq.AddCookie(sessionCookie)
	tokenReq.AddCookie(&http.Cookie{
		Name:  "login_session_swicus_org_id",
		Value: config.OrgID,
	})
	tokenResp, err := config.HTTPClient.Do(tokenReq)
	if err != nil {
		return nil, err
	}
	defer tokenResp.Body.Close()

	jwtCookie, err := getCookie(tokenResp, "jwt")
	if err != nil {
		return nil, err
	}

	return &jwtCookie.Value, err
}

// NewRequest makes a new HTTP Request.
func (pc *Client) NewRequest(method string, rsc string, params map[string]string) (*http.Request, error) {
	baseURL, err := url.Parse(pc.BaseURL.String() + rsc)
	if err != nil {
		return nil, err
	}

	if params != nil {
		ps := url.Values{}
		for k, v := range params {
			ps.Set(k, v)
		}
		baseURL.RawQuery = ps.Encode()
	}

	req, err := http.NewRequest(method, baseURL.String(), nil)
	if err != nil {
		return nil, err
	}

	// Use appropriate authentication method
	if pc.useAPITokenOnly {
		// Use API token authentication with Bearer token as per Pingdom API 3.1
		req.Header.Add("Authorization", "Bearer "+pc.APITokenOnly)
	} else {
		// Use JWT token authentication
		req.AddCookie(&http.Cookie{
			Name:  "jwt",
			Value: pc.JWTToken,
		})
	}

	return req, err
}

// Do makes an HTTP request and will unmarshal the JSON response in to the
// passed in interface.
func (pc *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := pc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := validateResponse(resp); err != nil {
		return resp, err
	}

	err = decodeResponse(resp, v)
	return resp, err
}

func decodeResponse(r *http.Response, v interface{}) error {
	if v == nil {
		return fmt.Errorf("nil interface provided to decodeResponse")
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	err := json.Unmarshal([]byte(bodyString), &v)
	return err
}

// Takes an HTTP response and determines whether it was successful.
func validateResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	bodyString := string(bodyBytes)
	m := &errorJSONResponse{}
	err := json.Unmarshal([]byte(bodyString), &m)
	if err != nil {
		return err
	}

	return m.Error
}

func getCookie(resp *http.Response, name string) (*http.Cookie, error) {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie, nil
		}
	}

	return nil, errors.New("there is no cookie in the response")
}
