package pingdomext

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sam-ijegs/go-pingdom/pingdom"
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
	client = &Client{
		JWTToken: "my_jwt_token",
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Integrations: nil,
	}
	client.Integrations = &IntegrationService{client: client}

	url, _ := url.Parse(server.URL)
	client.BaseURL = url
}

func teardown() {
	server.Close()
}

func TestNewClientWithConfig(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if m := "GET"; m != r.Method {
			t.Errorf("Request method = %v, want %v", r.Method, m)
		}
		w.Header().Add("Set-Cookie", "pingdom_login_session_id=qw4us4Ed7aLSGugMRDHkqM9G6mwuKdn9Hz90r6IHhRc%3D; Path=/; HttpOnly; Secure")
		w.Header().Add("Location", "https://my.solarwinds.cloud/login?response_type=code&scope=openid%20swicus&client_id=pingdom&state=htILEppzoMPtb6UjOdM98XPS3Mcwkr3Y&redirect_uri=https%3A%2F%2Fmy.pingdom.com%2Fauth%2Fswicus%2Fcallback")
		_, _ = fmt.Fprintf(w, "{}")
	})

	mux.HandleFunc("/v1/login", func(w http.ResponseWriter, r *http.Request) {
		if m := "POST"; m != r.Method {
			t.Errorf("Request method = %v, want %v", r.Method, m)
		}
		_, _ = fmt.Fprintf(w,
			`{"RedirectUrl": "https://my.pingdom.com/auth/swicus/callback?code=70kRkkAB7OIv5YYTPR6LpHH-2jMbtaDEHScLDw1amfw.baMoW3w-HkNXOj_I8pv580mRwBjIRVdFLW3cXFGRX9o&scope=openid+swicus&state=htILEppzoMPtb6UjOdM98XPS3Mcwkr3Y"}`,
		)
	})

	mux.HandleFunc("/auth/swicus/callback", func(w http.ResponseWriter, r *http.Request) {
		if m := "GET"; m != r.Method {
			t.Errorf("Request method = %v, want %v", r.Method, m)
		}
		w.Header().Add("Set-Cookie", "jwt=my_test_token")
		_, _ = fmt.Fprintf(w, "{}")
	})

	url, err := url.Parse(server.URL)
	assert.NotEmpty(t, url)
	assert.NoError(t, err)

	c, err := NewClientWithConfig(ClientConfig{
		Username: "test_user",
		Password: "test_pwd",
		OrgID:    "test_org",
		BaseURL:  url.String(),
		AuthURL:  url.String() + "/v1/login",
		HTTPClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, c.JWTToken, "my_test_token")
	assert.NotNil(t, c.Integrations)
}

func TestNewClientWithConfig2(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if m := "GET"; m != r.Method {
			t.Errorf("Request method = %v, want %v", r.Method, m)
		}
		w.Header().Add("Set-Cookie", "pingdom_login_session_id=qw4us4Ed7aLSGugMRDHkqM9G6mwuKdn9Hz90r6IHhRc%3D; Path=/; HttpOnly; Secure")
		w.Header().Add("Location", "https://my.solarwinds.cloud/login?response_type=code&scope=openid%20swicus&client_id=pingdom&state=htILEppzoMPtb6UjOdM98XPS3Mcwkr3Y&redirect_uri=https%3A%2F%2Fmy.pingdom.com%2Fauth%2Fswicus%2Fcallback")
		_, _ = fmt.Fprintf(w, "{}")
	})

	url, err := url.Parse(server.URL)
	assert.NotEmpty(t, url)
	assert.NoError(t, err)

	c, err := NewClientWithConfig(ClientConfig{
		Username: "test_user",
		Password: "test_pwd",
		OrgID:    "test_org",
		BaseURL:  url.String(),
		AuthURL:  url.String() + "/v1/login",
		HTTPClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	})
	assert.Error(t, err)
	assert.Nil(t, c)
}

// Add this test to verify client creation with API token
func TestNewClientWithAPIToken(t *testing.T) {
	setup()
	defer teardown()

	url, err := url.Parse(server.URL)
	assert.NotEmpty(t, url)
	assert.NoError(t, err)

	// Test creating client with API token
	c, err := NewClientWithConfig(ClientConfig{
		APITokenOnly: "test_api_token",
		BaseURL:      url.String(),
		HTTPClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "test_api_token", c.APITokenOnly)
	assert.Equal(t, true, c.useAPITokenOnly)
	assert.NotNil(t, c.Integrations)
	assert.Empty(t, c.JWTToken) // Should not have JWT token
}

// Add this test to verify requests with API token authentication
func TestClientAPITokenNewRequest(t *testing.T) {
	setup()
	defer teardown()

	// Create a client with API token
	apiClient := &Client{
		APITokenOnly:    "test_api_token",
		useAPITokenOnly: true,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Integrations: nil,
	}
	apiClient.Integrations = &IntegrationService{client: apiClient}

	url, _ := url.Parse(server.URL)
	apiClient.BaseURL = url

	// Test creating a request with API token auth
	req, err := apiClient.NewRequest("GET", "/data/v3/integration", nil)

	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, apiClient.BaseURL.String()+"/data/v3/integration", req.URL.String())
	
	// Verify Authorization header contains the Bearer token
	authHeader := req.Header.Get("Authorization")
	assert.Equal(t, "Bearer test_api_token", authHeader)
}

// Add this test to verify backward compatibility with JWT token
func TestClientJWTTokenNewRequest(t *testing.T) {
	setup()
	defer teardown()

	// Create a client with JWT token (old method)
	jwtClient := &Client{
		JWTToken:        "test_jwt_token",
		useAPITokenOnly: false,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Integrations: nil,
	}
	jwtClient.Integrations = &IntegrationService{client: jwtClient}

	url, _ := url.Parse(server.URL)
	jwtClient.BaseURL = url

	// Test creating a request with JWT token auth
	req, err := jwtClient.NewRequest("GET", "/data/v3/integration", nil)

	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, jwtClient.BaseURL.String()+"/data/v3/integration", req.URL.String())
	
	// Verify JWT token is sent as a cookie
	cookies := req.Cookies()
	assert.Equal(t, 1, len(cookies))
	assert.Equal(t, "jwt", cookies[0].Name)
	assert.Equal(t, "test_jwt_token", cookies[0].Value)
}

func TestClient_NewRequest(t *testing.T) {
	setup()
	defer teardown()

	req, err := client.NewRequest("GET", "/data/v3/integration", nil)

	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, client.BaseURL.String()+"/data/v3/integration", req.URL.String())
	
	// Verify the request has the JWT cookie and not the Authorization header
	authHeader := req.Header.Get("Authorization")
	assert.Empty(t, authHeader)
	
	cookies := req.Cookies()
	assert.Equal(t, 1, len(cookies))
	assert.Equal(t, "jwt", cookies[0].Name)
	assert.Equal(t, "my_jwt_token", cookies[0].Value)
}

func TestClient_Do(t *testing.T) {
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

func Test_decodeResponse(t *testing.T) {
	type args struct {
		r *http.Response
		v interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				r: &http.Response{
					Body: ioutil.NopCloser(strings.NewReader(`
					{
						"integration": {
							"status": true,
							"id": 112396
						}
					}`)),
				},
				v: &integrationJSONResponse{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := decodeResponse(tt.args.r, tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("decodeResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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

	want := &pingdom.PingdomError{StatusCode: 400, StatusDesc: "Bad Request", Message: "This is an error"}
	assert.Equal(t, want, validateResponse(invalid))
}

func Test_getCookie(t *testing.T) {
	type args struct {
		resp *http.Response
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    *http.Cookie
		wantErr bool
	}{
		{
			name: "response with session cookie",
			args: args{
				name: "pingdom_login_session_id",
				resp: &http.Response{
					Header: http.Header{
						"Set-Cookie": {"pingdom_login_session_id=xxxxxxx", "Path=/", "HttpOnly", "Secure"},
					},
				},
			},
			want: &http.Cookie{
				Name:  "pingdom_login_session_id",
				Value: "xxxxxxx",
				Raw:   "pingdom_login_session=xxxxxxx",
			},
			wantErr: false,
		},
		{
			name: "response without session cookie",
			args: args{
				name: "pingdom_login_session_id",
				resp: &http.Response{
					Header: http.Header{
						"Set-Cookie": {"pingdom_login_session=xxxxxxx", "Path=/", "HttpOnly", "Secure"},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCookie(tt.args.resp, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCookie() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.String() != tt.want.String() {
				t.Errorf("getCookie() = %v, want %v", got, tt.want)
			}
		})
	}
}
