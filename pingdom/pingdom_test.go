package pingdom

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	mux    *http.ServeMux
	client *Client
	server *httptest.Server
)

func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// test client
	client, _ = NewClientWithConfig(ClientConfig{
		APIToken: "my_api_token",
	})

	url, _ := url.Parse(server.URL)
	client.BaseURL = url
}

func setupWithAPITokenOnly() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// test client
	client, _ = NewClientWithConfig(ClientConfig{
		APITokenOnly: "my_api_token_only",
	})

	url, _ := url.Parse(server.URL)
	client.BaseURL = url
}

func teardown() {
	server.Close()
}

func testMethod(t *testing.T, r *http.Request, want string) {
	assert.Equal(t, want, r.Method)
}

func TestNewClientWithConfig(t *testing.T) {
	// Test with API Token
	c, err := NewClientWithConfig(ClientConfig{
		APIToken: "token",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, "token", c.APIToken)
	assert.Equal(t, "", c.APITokenOnly)

	// Test with API Token Only
	c, err = NewClientWithConfig(ClientConfig{
		APITokenOnly: "token_only",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, "", c.APIToken)
	assert.Equal(t, "token_only", c.APITokenOnly)

	// Test with both API Token and API Token Only
	c, err = NewClientWithConfig(ClientConfig{
		APIToken: "token",
		APITokenOnly: "token_only",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, "token", c.APIToken)
	assert.Equal(t, "token_only", c.APITokenOnly)

	// Test with no credentials
	c, err = NewClientWithConfig(ClientConfig{})
	assert.Error(t, err)
}

func TestNewClientWithEnvAPITokenDoesNotOverride(t *testing.T) {
	os.Setenv("PINGDOM_API_TOKEN", "envSetToken")
	defer os.Unsetenv("PINGDOM_API_TOKEN")

	c, err := NewClientWithConfig(ClientConfig{
		APIToken: "explicitToken",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, c.APIToken, "explicitToken")
}

func TestNewClientWithEnvAPITokenOnlyDoesNotOverride(t *testing.T) {
	os.Setenv("PINGDOM_API_TOKEN_ONLY", "envSetTokenOnly")
	defer os.Unsetenv("PINGDOM_API_TOKEN_ONLY")

	c, err := NewClientWithConfig(ClientConfig{
		APITokenOnly: "explicitTokenOnly",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, c.APITokenOnly, "explicitTokenOnly")
}

func TestNewClientWithEnvAPITokenWorks(t *testing.T) {
	os.Setenv("PINGDOM_API_TOKEN", "envSetToken")
	defer os.Unsetenv("PINGDOM_API_TOKEN")

	c, err := NewClientWithConfig(ClientConfig{})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, c.APIToken, "envSetToken")
}

func TestNewClientWithEnvAPITokenOnlyWorks(t *testing.T) {
	os.Setenv("PINGDOM_API_TOKEN_ONLY", "envSetTokenOnly")
	defer os.Unsetenv("PINGDOM_API_TOKEN_ONLY")
	
	// Clear API token in case it's also set
	origToken := os.Getenv("PINGDOM_API_TOKEN")
	os.Unsetenv("PINGDOM_API_TOKEN")
	defer os.Setenv("PINGDOM_API_TOKEN", origToken)

	c, err := NewClientWithConfig(ClientConfig{})
	assert.NoError(t, err)
	assert.Equal(t, http.DefaultClient, c.client)
	assert.Equal(t, defaultBaseURL, c.BaseURL.String())
	assert.NotNil(t, c.Checks)
	assert.Equal(t, c.APITokenOnly, "envSetTokenOnly")
}

func TestClientAuthenticationHeaders(t *testing.T) {
	// Test API Token authentication header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clientWithToken, _ := NewClientWithConfig(ClientConfig{
		APIToken: "token",
	})
	baseURL, _ := url.Parse(server.URL)
	clientWithToken.BaseURL = baseURL
	req, _ := clientWithToken.NewRequest("GET", "/", nil)
	clientWithToken.Do(req, nil)

	// Test API Token Only authentication header
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token_only", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clientWithTokenOnly, _ := NewClientWithConfig(ClientConfig{
		APITokenOnly: "token_only",
	})
	baseURL, _ = url.Parse(server.URL)
	clientWithTokenOnly.BaseURL = baseURL
	req, _ = clientWithTokenOnly.NewRequest("GET", "/", nil)
	clientWithTokenOnly.Do(req, nil)

	// Test precedence (API Token Only should be used if both are provided)
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token_only", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	clientWithBoth, _ := NewClientWithConfig(ClientConfig{
		APIToken: "token",
		APITokenOnly: "token_only",
	})
	baseURL, _ = url.Parse(server.URL)
	clientWithBoth.BaseURL = baseURL
	req, _ = clientWithBoth.NewRequest("GET", "/", nil)
	clientWithBoth.Do(req, nil)
}

func TestNewRequest(t *testing.T) {
	setup()
	defer teardown()

	req, err := client.NewRequest("GET", "/checks", nil)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, client.BaseURL.String()+"/checks", req.URL.String())
}

func TestNewRequestWithAPITokenOnly(t *testing.T) {
	setupWithAPITokenOnly()
	defer teardown()

	req, err := client.NewRequest("GET", "/checks", nil)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, client.BaseURL.String()+"/checks", req.URL.String())
	assert.Equal(t, "Bearer my_api_token_only", req.Header.Get("Authorization"))
}

func TestDo(t *testing.T) {
	setup()
	defer teardown()

	type foo struct {
		A string
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if m := "GET"; m != r.Method {
			t.Errorf("Request method = %v, want %v", r.Method, m)
		}
		fmt.Fprint(w, `{"A":"a"}`)
	})

	req, _ := client.NewRequest("GET", "/", nil)
	body := new(foo)
	want := &foo{"a"}
	_, err := client.Do(req, body)
	assert.NoError(t, err)
	assert.Equal(t, want, body)
}

func TestValidateResponse(t *testing.T) {
	valid := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}
	assert.NoError(t, validateResponse(valid))

	invalid := &http.Response{
		Request:    &http.Request{},
		StatusCode: http.StatusBadRequest,
		Body: ioutil.NopCloser(strings.NewReader(`{
		"error" : {
			"statuscode": 400,
			"statusdesc": "Bad Request",
			"errormessage": "This is an error"
		}
		}`)),
	}
	want := &PingdomError{400, "Bad Request", "This is an error"}
	assert.Equal(t, want, validateResponse(invalid))
}
