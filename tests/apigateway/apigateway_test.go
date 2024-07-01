package apigateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
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

	dialer := &net.Dialer{}
	s.tr = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.Contains(addr, "docker.localhost") {
				addr = s.lbIP + ":80"
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
	err = testhelpers.WaitForPodReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)
	s.apply("api-gateway/1-getting-started/manifests/weather-app-ingressroute.yaml")

	req, err := http.NewRequest(http.MethodGet, "http://getting-started.apigateway.docker.localhost", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 30*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/1-getting-started/manifests/weather-app-apikey.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://getting-started.apigateway.docker.localhost/api-key", nil)
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

func (s *APIGatewayTestSuite) TestSecureApplications() {
	var err error
	var req *http.Request

	s.apply("src/manifests/hydra.yaml")
	err = testhelpers.WaitForPodReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=hydra")
	s.Require().NoError(err)
	err = testhelpers.WaitForPodReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=consent")
	s.Require().NoError(err)
	err = testhelpers.WaitForJobCompleted(s.ctx, s.T(), s.k8s, 60*time.Second, "app=create-hydra-clients")
	s.Require().NoError(err)

	// Test M2M
	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("src/manifests/whoami-app.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=whoami")
	s.Require().NoError(err)
	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-ingressroute.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/no-auth", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml")
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
	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-oauth2-client-creds-nologin.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/oauth2-client-credentials-nologin", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// Test OIDC
	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-ingressroute.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://secure-applications.apigateway.docker.localhost/no-auth", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-oidc.yaml")

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
	s.apply("api-gateway/2-secure-applications/manifests/whoami-app-oidc-nologinurl.yaml")

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
