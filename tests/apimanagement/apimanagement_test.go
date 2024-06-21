package apimanagement

import (
	"context"
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
	"github.com/traefik/hub-preview/tests/testhelpers"
	"github.com/traefik/traefik/v3/integration/try"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type APIManagementTestSuite struct {
	suite.Suite
	k8s  client.Client
	k3d  *k3s.K3sContainer
	ctx  context.Context
	tr   *http.Transport
	lbIP string
}

const kubeConfigEnvVar = "KUBECONFIG"

func (s *APIManagementTestSuite) SetupSuite() {
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

	s.lbIP, err = testhelpers.InstallTraefikHubAPIM(s.ctx, s.T(), s.k8s)
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
	envVariables := []string{"ADMIN_TOKEN", "API_TOKEN", "EXTERNAL_TOKEN", "PLATFORM_URL"}
	for _, envVariable := range envVariables {
		if os.Getenv(envVariable) == "" {
			return errors.New("Required env variable: " + envVariable)
		}
	}
	return nil
}

// TearDown is done using t.Cleanup()
func (s *APIManagementTestSuite) TearDownSuite() {}

func (s *APIManagementTestSuite) TestDashboardAccess() {
	err := s.check(http.MethodGet, "http://dashboard.docker.localhost/dashboard/", 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestGettingStarted() {
	var err error
	adminToken := os.Getenv("ADMIN_TOKEN")

	s.apply("src/manifests/weather-app.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitFor(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/weather-app-ingressroute.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://getting-started.apimanagement.docker.localhost", 10*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/api.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost/weather", 10*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/api-portal.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost", 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost/weather", adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestAccessControl() {
	var err error
	externalToken, adminToken := os.Getenv("EXTERNAL_TOKEN"), os.Getenv("ADMIN_TOKEN")

	// Simple Access Control
	s.apply("src/manifests/weather-app.yaml")
	s.apply("src/manifests/admin-app.yaml")
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitFor(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)
	err = testhelpers.WaitFor(s.ctx, s.T(), s.k8s, 90*time.Second, "app=admin-app")
	s.Require().NoError(err)

	s.apply("api-management/2-access-control/manifests/simple-admin-api.yaml")
	s.apply("api-management/2-access-control/manifests/simple-weather-api.yaml")

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/admin", adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/weather", adminToken, 5*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/weather", externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/admin", externalToken, 5*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)

	// Complex Access Control
	s.apply("api-management/2-access-control/manifests/complex-admin-api.yaml")
	s.apply("api-management/2-access-control/manifests/complex-weather-api.yaml")

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/admin", adminToken, 10*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/weather", adminToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather", adminToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/weather", externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather", externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = testhelpers.Delete(s.ctx, s.k8s, "APIAccess", "access-control-apimanagement-simple-weather", "apps", "hub.traefik.io", "v1alpha1")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather", externalToken, 10*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestAPILifeCycleManagement() {
	var err error
	adminToken := os.Getenv("ADMIN_TOKEN")

	// Publish First API Version
	err = s.apply("src/manifests/weather-app.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api.yaml")
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	err = testhelpers.WaitFor(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)

	err = s.check(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather", 5*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)
	err = s.checkWithBearer(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather", adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)

	err = s.check(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-v1", 5*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)
	err = s.checkWithBearer(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-v1", adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	// Publish Second API Version
	err = s.apply("src/manifests/weather-app-forecast.yaml")
	s.Assert().NoError(err)
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml")
	s.Assert().NoError(err)

	time.Sleep(1 * time.Second)
	err = testhelpers.WaitFor(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app-forecast")
	s.Require().NoError(err)

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions", nil)
	s.Require().NoError(err)
	req.Header.Add("X-Version", "preview")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("weather"))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions", nil)
	s.Require().NoError(err)
	req.Header.Add("X-Version", "preview")
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("forecast"))
	s.Assert().NoError(err)

	// Try the new version with a part of the traffic
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml")
	s.Require().NoError(err)

	// both should work
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("forecast"))
	s.Assert().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("weather"))
	s.Assert().NoError(err)

}

func TestAPIManagementTestSuite(t *testing.T) {
	suite.Run(t, new(APIManagementTestSuite))
}

func (s *APIManagementTestSuite) apply(path string) error {
	results, err := testhelpers.ApplyFile(s.ctx, s.k8s, filepath.Join("..", "..", path))
	if err != nil {
		return err
	}
	testcontainers.Logger.Printf("📦️ %q loaded\n", results)
	return err
}

func (s *APIManagementTestSuite) check(method string, url string, timeout time.Duration, status int) error {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}
	return try.RequestWithTransport(req, timeout, s.tr, try.StatusCodeIs(status))
}

func (s *APIManagementTestSuite) checkWithBearer(method, url, bearer string, timeout time.Duration, status int) error {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+bearer)
	return try.RequestWithTransport(req, timeout, s.tr, try.StatusCodeIs(status))
}
