package walkthrough

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
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

type WalkthroughTestSuite struct {
	suite.Suite
	k8s client.Client
	k3d *k3s.K3sContainer
	ctx context.Context
	tr  *http.Transport
}

const kubeConfigEnvVar = "KUBECONFIG"

func (s *WalkthroughTestSuite) SetupSuite() {
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

	lbIP, err := testhelpers.InstallTraefikProxy(s.ctx, s.T(), s.k8s)
	s.Require().NoError(err)

	dialer := &net.Dialer{}
	s.tr = &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.Contains(addr, "docker.localhost") {
				addr = lbIP + ":80"
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
func (s *WalkthroughTestSuite) TearDownSuite() {}

func (s *WalkthroughTestSuite) TestDashboardAccess() {
	req, err := http.NewRequest(http.MethodGet, "http://dashboard.docker.localhost/dashboard/", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 2*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

func (s *WalkthroughTestSuite) TestWalkthrough() {
	// STEP 1
	s.apply("src/manifests/apps-namespace.yaml")
	s.apply("src/manifests/weather-app.yaml")
	s.apply("src/manifests/walkthrough/weather-app-no-auth.yaml")

	req, err := http.NewRequest(http.MethodGet, "http://walkthrough.docker.localhost/no-auth/weather", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("src/manifests/walkthrough/weather-app-basic-auth.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://walkthrough.docker.localhost/basic-auth/weather", nil)
	s.Require().NoError(err)

	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	req.SetBasicAuth("foo", "bar")
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// STEP 2
	testhelpers.CreateSecretForTraefikHub(s.ctx, s.T(), s.k8s)
	testhelpers.LaunchHelmCommand(s.T(), "upgrade", "traefik", "-n", "traefik", "--wait",
		"--version", "v31.1.1",
		"--reuse-values",
		"--set", "hub.token=traefik-hub-license",
		"--set", "image.registry=ghcr.io",
		"--set", "image.repository=traefik/traefik-hub",
		"--set", "image.tag=v3.4.1",
		"traefik/traefik")

	req, err = http.NewRequest(http.MethodGet, "http://walkthrough.docker.localhost/basic-auth/weather", nil)
	s.Require().NoError(err)

	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)
	req.SetBasicAuth("foo", "bar")
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("src/manifests/walkthrough/weather-app-apikey.yaml")
	req, err = http.NewRequest(http.MethodGet, "http://walkthrough.docker.localhost/api-key/weather", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	apiKey := base64.StdEncoding.EncodeToString([]byte("Let's use API Key with Traefik Hub"))
	req.Header.Add("Authorization", "Bearer "+apiKey)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	// STEP 3
	testhelpers.LaunchHelmCommand(s.T(), "upgrade", "traefik", "-n", "traefik", "--wait",
		"--version", "v31.1.1",
		"--reuse-values",
		"--set", "hub.apimanagement.enabled=true",
		"traefik/traefik")

	req, err = http.NewRequest(http.MethodGet, "http://walkthrough.docker.localhost/api-key/weather", nil)
	s.Require().NoError(err)

	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)
	req.Header.Add("Authorization", "Bearer "+apiKey)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("src/manifests/walkthrough/api.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://api.walkthrough.docker.localhost/weather", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 5*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	s.apply("src/manifests/walkthrough/api-portal.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://api.walkthrough.docker.localhost", nil)
	s.Require().NoError(err)
	err = try.RequestWithTransport(req, 90*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.walkthrough.docker.localhost/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+os.Getenv("ADMIN_TOKEN"))

	err = try.RequestWithTransport(req, 90*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)

	s.apply("src/manifests/weather-app-forecast.yaml")
	s.apply("src/manifests/walkthrough/forecast.yaml")

	req, err = http.NewRequest(http.MethodGet, "http://api.walkthrough.docker.localhost/forecast/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+os.Getenv("ADMIN_TOKEN"))

	err = try.RequestWithTransport(req, 90*time.Second, s.tr, try.StatusCodeIs(http.StatusOK))
	s.Assert().NoError(err)
}

func TestWalkthroughTestSuite(t *testing.T) {
	suite.Run(t, new(WalkthroughTestSuite))
}

func (s *WalkthroughTestSuite) apply(path string) {
	results, err := testhelpers.ApplyFile(s.ctx, s.k8s, filepath.Join("..", "..", path))
	s.Require().NoError(err)
	testcontainers.Logger.Printf("📦️ %q loaded\n", results)
}
