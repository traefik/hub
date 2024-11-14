package apigateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"github.com/traefik/hub/tests/testhelpers"
	"github.com/traefik/traefik/v3/integration/try"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type APIGatewayTestSuite struct {
	suite.Suite
	k8s   client.Client
	k3d   *k3s.K3sContainer
	ctx   context.Context
	tr    *http.Transport
	lbIP  string
	debug bool
}

const kubeConfigEnvVar = "KUBECONFIG"

func (s *APIGatewayTestSuite) SetupSuite() {
	s.ctx = context.Background()

	err := checkRequiredEnvVariables()
	s.Require().NoError(err)

	k3d, err := testhelpers.CreateKubernetesCluster(s.ctx, s.T())
	s.Require().NoError(err)

	s.k3d = k3d
	kubeConfigYaml, err := k3d.GetKubeConfig(s.ctx)
	s.Require().NoError(err)

	f, err := os.CreateTemp(s.T().TempDir(), "kbcfg-")
	s.Require().NoError(err)
	defer f.Close()

	_, err = f.Write(kubeConfigYaml)
	s.Require().NoError(err)

	s.T().Setenv(kubeConfigEnvVar, f.Name())
	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	s.Require().NoError(err)

	s.k8s, err = client.New(restcfg, client.Options{})
	s.Require().NoError(err)

	testhelpers.LaunchHelmCommand(s.T(), "repo", "add", "--force-update", "traefik", "https://traefik.github.io/charts")

	s.lbIP, err = testhelpers.InstallTraefikHubAPIGW(s.ctx, s.T(), s.k8s)
	s.Require().NoError(err)

	re := regexp.MustCompile(`([-a-z.]+\.docker\.localhost)`)
	dialer := &net.Dialer{}
	s.tr = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.Contains(addr, "docker.localhost") {
				new_addr := re.ReplaceAllString(addr, s.lbIP)
				// testcontainers.Logger.Printf("addr: %s => new_addr: %s\n", addr, new_addr)
				addr = new_addr
			}
			return dialer.DialContext(ctx, network, addr)
		},
	}
}

func checkRequiredEnvVariables() error {
	envVariables := []string{"ADMIN_TOKEN", "API_TOKEN", "PLATFORM_URL"}
	for _, envVariable := range envVariables {
		if os.Getenv(envVariable) == "" {
			return errors.New("Required env variable: " + envVariable)
		}
	}
	return nil
}

// TearDown is done using t.Cleanup()
func (s *APIGatewayTestSuite) TearDownSuite() {}

func (s *APIGatewayTestSuite) TestDashboardAccess() {
	req, err := http.NewRequest(http.MethodGet, "http://dashboard.docker.localhost/dashboard/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 2*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

func (s *APIGatewayTestSuite) TestGettingStarted() {
	var err error

	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("src/manifests/weather-app.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)
	s.apply("api-gateway/1-getting-started/manifests/weather-app-ingressroute.yaml")

	req, err := http.NewRequest(http.MethodGet, "http://getting-started.apigateway.docker.localhost/weather", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 30*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/1-getting-started/manifests/weather-app-apikey.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://getting-started.apigateway.docker.localhost/api-key/weather", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	apiKey := base64.StdEncoding.EncodeToString([]byte("Let's use API Key with Traefik Hub"))
	req.Header.Add("Authorization", "Bearer "+apiKey)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

type k8sSecret struct {
	Data oAuth2ClientConfig `json:"data"`
}
type oAuth2ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	// No interest in other fields for this use case
}

func (s *APIGatewayTestSuite) TestExpose() {
	var err error
	var req *http.Request

	jsonBody := []byte(`{ "query": "{ continents { code name } }" }`)
	bodyReader := bytes.NewReader(jsonBody)
	req, err = http.NewRequest(http.MethodPost, "https://countries.trevorblades.com/", bodyReader)
	s.Require().NoError(err)
	req.Header.Add("content-type", "application/json")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("api-gateway/2-expose/manifests/graphql-service.yaml")
	s.apply("api-gateway/2-expose/manifests/graphql-ingressroute.yaml")
	// no way to check IngressRoute is loaded, FTM
	time.Sleep(1 * time.Second)

	bodyReader = bytes.NewReader(jsonBody)
	req, err = http.NewRequest(http.MethodPost, "http://expose.apigateway.docker.localhost/graphql", bodyReader)
	s.Require().NoError(err)
	req.Header.Add("content-type", "application/json")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusNotFound))
	s.Assert().NoError(err)

	testhelpers.LaunchHelmUpgradeCommand(s.T(),
		"--set", "providers.kubernetesCRD.allowExternalNameServices=true",
	)

	time.Sleep(1 * time.Second)
	bodyReader = bytes.NewReader(jsonBody)
	req, err = http.NewRequest(http.MethodPost, "http://expose.apigateway.docker.localhost/graphql", bodyReader)
	s.Require().NoError(err)
	req.Header.Add("content-type", "application/json")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusMisdirectedRequest))
	s.Assert().NoError(err)

	s.apply("api-gateway/2-expose/manifests/graphql-ingressroute-complete.yaml")
	time.Sleep(1 * time.Second)
	bodyReader = bytes.NewReader(jsonBody)
	req, err = http.NewRequest(http.MethodPost, "http://expose.apigateway.docker.localhost/graphql", bodyReader)
	s.Require().NoError(err)
	req.Header.Add("content-type", "application/json")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Expose a website
	s.apply("api-gateway/2-expose/manifests/webapp-db.yaml")
	s.apply("api-gateway/2-expose/manifests/webapp-api.yaml")
	s.apply("api-gateway/2-expose/manifests/webapp-front.yaml")
	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 30*time.Second, "app in (db,api,web)")
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Compress static text content
	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/app.js", nil)
	s.Require().NoError(err)
	req.Header.Add("Accept-Encoding", "gzip, deflate, bz, zstd")

	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		testhelpers.HasNotHeader("Content-Encoding"),
	)
	s.Assert().NoError(err)

	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-compress.yaml")
	time.Sleep(1 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/app.js", nil)
	s.Require().NoError(err)
	req.Header.Add("Accept-Encoding", "gzip, deflate, bz, zstd")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("Content-Encoding", "zstd", true),
	)
	s.Assert().NoError(err)

	// Protect with Security Headers and CORS
	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	req.Header.Add("Origin", "http://test2.com")

	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		testhelpers.HasNotHeader("Access-Control-Allow-Origin"),
	)
	s.Assert().NoError(err)

	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-cors.yaml")
	time.Sleep(1 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	req.Header.Add("Origin", "http://test.com")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("Access-Control-Allow-Origin", "http://test.com", true),
	)
	s.Assert().NoError(err)

	// Error page
	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/doesnotexist", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusNotFound),
		try.BodyContains("404 page not found"), // Proxy default content on 404
	)
	s.Assert().NoError(err)

	s.apply("api-gateway/2-expose/manifests/error-page.yaml")
	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-error-page.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 30*time.Second, "app=error-page")
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/doesnotexist", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusNotFound),
		try.BodyContains("Error 404: Not Found"), // Error page default content on 404
	)
	s.Assert().NoError(err)

	// HTTP Caching
	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-cache.yaml")
	time.Sleep(1 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Cache-Status", "MISS", true),
	)
	s.Assert().NoError(err)

	err = try.RequestWithTransport(req, 10*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Cache-Status", "HIT", true),
	)
	s.Assert().NoError(err)

	// Configure HTTPS
	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-https.yaml")
	time.Sleep(2 * time.Second)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "expose.apigateway.docker.localhost",
		NextProtos:         []string{"h2", "http/1.1"},
	}

	conn, err := tls.Dial("tcp", s.lbIP+":443", tlsConfig)
	s.Require().NoError(err)
	defer conn.Close()

	err = conn.Handshake()
	s.Require().NoError(err)
	cs := conn.ConnectionState()
	s.Require().Equal(cs.PeerCertificates[0].Issuer.CommonName, "TRAEFIK DEFAULT CERT")

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// TLS with CA certificates
	s.apply("src/minica/expose.apigateway.docker.localhost/expose.yaml")
	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-https-manual.yaml")
	time.Sleep(2 * time.Second)

	caCert, err := os.ReadFile(filepath.Join("..", "..", "src/minica/minica.pem"))
	s.Assert().NoError(err)
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	s.tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         "expose.apigateway.docker.localhost",
		RootCAs:            caCertPool,
	}

	// Check https with CA Cert
	req, err = http.NewRequest(http.MethodGet, "https://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	req.Header.Set("Host", s.tr.TLSClientConfig.ServerName)
	err = try.RequestWithTransport(req, 3*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Check http with CA Cert
	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusNotFound))
	s.Assert().NoError(err)

	// Check https on http with CA Cert
	req, err = http.NewRequest(http.MethodGet, "https://expose.apigateway.docker.localhost:80/", nil)
	s.Require().NoError(err)
	req.Header.Set("Host", s.tr.TLSClientConfig.ServerName)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-https-split.yaml")
	time.Sleep(1 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "https://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	req.Header.Set("Host", s.tr.TLSClientConfig.ServerName)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// TLS with automation
	s.apply("src/minica/pebble.pebble.svc/pebble.yaml")
	s.apply("api-gateway/2-expose/manifests/pebble.yaml")
	s.apply("src/minica/minica.yaml")
	s.apply("api-gateway/2-expose/manifests/coredns-config.yaml")

	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 30*time.Second, "app=pebble")
	s.Require().NoError(err)
	err = testhelpers.RestartDeployment(s.ctx, s.T(), s.k8s, "coredns", "kube-system")
	s.Require().NoError(err)

	testhelpers.LaunchHelmUpgradeCommand(s.T(),
		"--set", "certificatesResolvers.pebble.distributedAcme.caServer=https://pebble.pebble.svc:14000/dir",
		"--set", "certificatesResolvers.pebble.distributedAcme.email=test@example.com",
		"--set", "certificatesResolvers.pebble.distributedAcme.storage.kubernetes=true",
		"--set", "certificatesResolvers.pebble.distributedAcme.tlsChallenge=true",
		"--set", "volumes[0].name=minica",
		"--set", "volumes[0].mountPath=/minica",
		"--set", "volumes[0].type=secret",
		"--set", "env[0].name=LEGO_CA_CERTIFICATES",
		"--set", "env[0].value=/minica/minica.pem",
	)

	s.apply("api-gateway/2-expose/manifests/webapp-ingressroute-https-auto.yaml")
	time.Sleep(25 * time.Second)

	s.apply("api-gateway/2-expose/manifests/pebble-ingressroute.yaml")
	time.Sleep(5 * time.Second)

	caCertPool = x509.NewCertPool()
	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/pebble/roots/0", nil)
	s.Assert().NoError(err)
	response, err := http.DefaultClient.Do(req)
	s.Assert().NoError(err)
	body, err := io.ReadAll(response.Body)
	s.Assert().NoError(err)
	caCertPool.AppendCertsFromPEM(body)

	req, err = http.NewRequest(http.MethodGet, "http://expose.apigateway.docker.localhost/pebble/intermediates/0", nil)
	s.Assert().NoError(err)
	response, err = http.DefaultClient.Do(req)
	s.Assert().NoError(err)
	body, err = io.ReadAll(response.Body)
	s.Assert().NoError(err)
	caCertPool.AppendCertsFromPEM(body)

	s.tr.TLSClientConfig.RootCAs = caCertPool
	// Check with dynamic CA Chain from pebble
	req, err = http.NewRequest(http.MethodGet, "https://expose.apigateway.docker.localhost/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 15*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

func (s *APIGatewayTestSuite) TestSecureApplications() {
	var err error
	var req *http.Request

	s.apply("src/manifests/hydra.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 120*time.Second, "app=hydra")
	s.Require().NoError(err)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=consent")
	s.Require().NoError(err)
	err = testhelpers.WaitForJobCompleted(s.ctx, s.T(), s.k8s, 60*time.Second, "app=create-hydra-clients")
	s.Require().NoError(err)

	// Test M2M
	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("src/manifests/whoami-app.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=whoami")
	s.Require().NoError(err)
	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-ingressroute.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/no-auth", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml")
	output := testhelpers.LaunchKubectl(s.T(), "get", "secrets", "-n", "apps", "oauth-client", "-o", "json")
	s.Require().NotNil(output)
	oauth2 := k8sSecret{}
	err = json.NewDecoder(output).Decode(&oauth2)
	s.Require().NoError(err)

	clientID, err := base64.StdEncoding.DecodeString(oauth2.Data.ClientID)
	s.Require().NoError(err)

	clientSecret, err := base64.StdEncoding.DecodeString(oauth2.Data.ClientSecret)
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oauth2-client-credentials", nil)
	s.Require().NoError(err)
	auth := base64.StdEncoding.EncodeToString([]byte(string(clientID) + ":" + string(clientSecret)))
	req.Header.Add("Authorization", "Basic "+auth)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// M2M with clientID / clientSecret in the mdw
	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-oauth2-client-creds-nologin.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oauth2-client-credentials-nologin", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Test OIDC
	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-ingressroute.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/no-auth", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-oidc.yaml")

	// FTM: No way to check when oidc has been loaded
	time.Sleep(5 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oidc", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oidc/login", nil)
	s.Require().NoError(err)

	// Create manually client to get and use Cookies
	// CookieJar is needed to follow requests and store cookies in resp
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	s.Require().NoError(err)
	client := http.DefaultClient
	client.Transport = s.tr
	client.Jar = jar

	resp, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(http.StatusNoContent, resp.StatusCode)
	if s.debug {
		testcontainers.Logger.Printf("Cookies in resp after login: %q\n", resp.Cookies())
	}

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oidc", nil)
	s.Require().NoError(err)
	for _, cookie := range resp.Cookies() {
		req.AddCookie(cookie)
	}
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Test OIDC No Login
	s.apply("api-gateway/3-secure-applications/manifests/whoami-app-oidc-nologinurl.yaml")

	// FTM: No way to check when oidc has been loaded
	time.Sleep(5 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oidc-nologin", nil)
	s.Require().NoError(err)
	// Create manually client to get and use Cookies
	client = http.DefaultClient
	client.Transport = s.tr
	jar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: nil})
	s.Require().NoError(err)
	client.Jar = jar

	resp, err = client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	if s.debug {
		testcontainers.Logger.Printf("Cookies in resp on /: %q\n", resp.Cookies())
	}

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oidc-nologin", nil)
	s.Require().NoError(err)
	for _, cookie := range resp.Cookies() {
		req.AddCookie(cookie)
	}
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

func TestAPIGatewayTestSuite(t *testing.T) {
	suite.Run(t, new(APIGatewayTestSuite))
}

func (s *APIGatewayTestSuite) apply(path string) {
	results, err := testhelpers.ApplyFile(s.ctx, s.k8s, filepath.Join("..", "..", path))
	s.Require().NoError(err)
	testcontainers.Logger.Printf("📦️ %q loaded\n", results)
}
