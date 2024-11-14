package apimanagement

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"github.com/traefik/hub/tests/testhelpers"
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

	err = s.apply("src/manifests/apps-namespace.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/weather-app.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/weather-app-ingressroute.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://getting-started.apimanagement.docker.localhost/weather", 10*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/api.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost/weather", 10*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)

	err = s.apply("api-management/1-getting-started/manifests/api-portal.yaml")
	s.Assert().NoError(err)
	err = s.check(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost", 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.getting-started.apimanagement.docker.localhost/weather", http.NoBody, adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestAccessControl() {
	var err error
	externalToken, adminToken := os.Getenv("EXTERNAL_TOKEN"), os.Getenv("ADMIN_TOKEN")

	// Simple Access Control
	err = s.apply("src/manifests/apps-namespace.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/weather-app.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/admin-app.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=admin-app")
	s.Require().NoError(err)

	err = s.apply("api-management/2-access-control/manifests/simple-admin-api.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/2-access-control/manifests/simple-weather-api.yaml")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/admin", http.NoBody, adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/weather", http.NoBody, adminToken, 5*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/weather", http.NoBody, externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/simple/admin", http.NoBody, externalToken, 5*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)

	// Complex Access Control
	err = s.apply("api-management/2-access-control/manifests/complex-admin-api.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/2-access-control/manifests/complex-weather-api.yaml")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/admin", http.NoBody, adminToken, 10*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/weather", http.NoBody, adminToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather/0", bytes.NewReader([]byte(`[{"op": "replace", "path": "/city", "value": "GopherTown"}]`)), adminToken, 5*time.Second, http.StatusNoContent)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.access-control.apimanagement.docker.localhost/complex/weather", http.NoBody, externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather/0", bytes.NewReader([]byte(`[{"op": "replace", "path": "/weather", "value": "Cloudy"}]`)), externalToken, 5*time.Second, http.StatusNoContent)
	s.Assert().NoError(err)

	err = testhelpers.Delete(s.ctx, s.k8s, "APIAccess", "access-control-apimanagement-simple-weather", "apps", "hub.traefik.io", "v1alpha1")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodPatch, "http://api.access-control.apimanagement.docker.localhost/complex/weather/0", bytes.NewReader([]byte(`[{"op": "replace", "path": "/weather", "value": "Cloudy"}]`)), externalToken, 10*time.Second, http.StatusForbidden)
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestAPILifeCycleManagement() {
	var err error
	adminToken := os.Getenv("ADMIN_TOKEN")

	// Publish First API Version
	err = s.apply("src/manifests/apps-namespace.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/weather-app.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api.yaml")
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)

	err = s.check(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather", 5*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)
	err = s.checkWithBearer(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather", http.NoBody, adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)

	err = s.check(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-v1/weather", 5*time.Second, http.StatusUnauthorized)
	s.Assert().NoError(err)
	err = s.checkWithBearer(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-v1/weather", http.NoBody, adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	// Publish Second API Version
	err = s.apply("src/manifests/weather-app-forecast.yaml")
	s.Assert().NoError(err)
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml")
	s.Assert().NoError(err)

	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app-forecast")
	s.Require().NoError(err)

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("X-Version", "preview")
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.StatusCodeIs(http.StatusUnauthorized))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("GopherRocks"))
	s.Assert().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("X-Version", "preview")
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("GopherCentral"))
	s.Assert().NoError(err)

	// Try the new version with a part of the traffic
	err = s.apply("api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml")
	s.Require().NoError(err)

	// both should work
	req, err = http.NewRequest(http.MethodGet, "http://api.lifecycle.apimanagement.docker.localhost/weather-v1-wrr/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("GopherRocks"))
	s.Assert().NoError(err)
	err = try.RequestWithTransport(req, 10*time.Second, s.tr, try.BodyContains("GopherCentral"))
	s.Assert().NoError(err)
}

func (s *APIManagementTestSuite) TestProtectAPIInfrastructure() {
	var err error
	var req *http.Request
	externalToken, adminToken := os.Getenv("EXTERNAL_TOKEN"), os.Getenv("ADMIN_TOKEN")

	// Step 1: Deploy `weather` and `admin` app
	err = s.apply("src/manifests/apps-namespace.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/weather-app.yaml")
	s.Require().NoError(err)
	err = s.apply("src/manifests/admin-app.yaml")
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=weather-app")
	s.Require().NoError(err)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=admin-app")
	s.Require().NoError(err)

	// Step 1: Deploy weather and admin APIs
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/admin-api.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/admin-apiaccess.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/admin-ingressroute.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-api.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-apiaccess.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-ingressroute.yaml")
	s.Require().NoError(err)

	// Protect from Excessive usage: step 1: set up redis
	testhelpers.LaunchHelmCommand(s.T(), "install", "redis", "oci://registry-1.docker.io/bitnamicharts/redis", "-n", "traefik", "--wait")

	sec := &corev1.Secret{}
	err = s.k8s.Get(s.ctx, client.ObjectKey{Namespace: "traefik", Name: "redis"}, sec)
	assert.NoError(s.T(), err)
	redisPassword := string(sec.Data["redis-password"])

	testcontainers.Logger.Printf("🔐 Found this redis password: %s\n", redisPassword)

	testhelpers.LaunchHelmUpgradeCommand(s.T(),
		"--set", "hub.redis.endpoints=redis-master.traefik.svc.cluster.local:6379",
		"--set", "hub.redis.password="+redisPassword,
	)

	// wait for account to be sync'd
	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/admin", http.NoBody, adminToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)
	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", http.NoBody, externalToken, 90*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	// RateLimit on Admin
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/admin-apiplan.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/admin-apiaccess-ratelimit.yaml")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/admin", http.NoBody, adminToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = testhelpers.Delete(s.ctx, s.k8s, "APIAccess", "protect-api-infrastructure-apimanagement-admin", "admin", "hub.traefik.io", "v1alpha1")
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	req, err = http.NewRequest(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/admin", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+adminToken)
	err = try.RequestWithTransport(req, 500*time.Millisecond, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Ratelimit-Remaining", "0", true),
	)
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/admin", http.NoBody, adminToken, 500*time.Millisecond, http.StatusTooManyRequests)
	s.Assert().NoError(err)

	// Quota on external API
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-apiplan.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-apiaccess-quota.yaml")
	s.Require().NoError(err)

	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", http.NoBody, externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = testhelpers.Delete(s.ctx, s.k8s, "APIAccess", "protect-api-infrastructure-apimanagement-weather", "apps", "hub.traefik.io", "v1alpha1")
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	for i := 0; i < 5; i++ {
		req, err = http.NewRequest(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", nil)
		s.Require().NoError(err)
		req.Header.Add("Authorization", "Bearer "+externalToken)
		err = try.RequestWithTransport(req, 500*time.Millisecond, s.tr,
			try.StatusCodeIs(http.StatusOK),
			try.HasHeaderValue("X-Quota-Remaining", strconv.Itoa(4-i), true),
		)
		s.Require().NoError(err)
	}

	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", http.NoBody, externalToken, 5*time.Second, http.StatusTooManyRequests)
	s.Assert().NoError(err)

	// Premium plan with higher quota
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-apiplan-premium.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/weather-apiaccess-quota-premium.yaml")
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+externalToken)
	err = try.RequestWithTransport(req, 2*time.Second, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Quota-Remaining", "494", true),
	)
	s.Require().NoError(err)

	// API Bundle
	err = s.apply("src/manifests/whoami-app.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)

	err = s.apply("api-management/4-protect-api-infrastructure/manifests/whoami-ingressroute.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/whoami-api.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/whoami-apiaccess.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)
	err = testhelpers.WaitForPodsReady(s.ctx, s.T(), s.k8s, 90*time.Second, "app=whoami")

	err = s.checkWithBearer(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/whoami", http.NoBody, externalToken, 5*time.Second, http.StatusOK)
	s.Assert().NoError(err)

	err = s.apply("api-management/4-protect-api-infrastructure/manifests/api-bundle.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/api-plan-for-bundle.yaml")
	s.Require().NoError(err)
	err = s.apply("api-management/4-protect-api-infrastructure/manifests/api-bundle-access.yaml")
	s.Require().NoError(err)
	time.Sleep(1 * time.Second)

	req, err = http.NewRequest(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/whoami", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+externalToken)
	err = try.RequestWithTransport(req, 500*time.Millisecond, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Quota-Remaining", "499", true),
		try.HasHeaderValue("X-Ratelimit-Remaining", "0", true),
	)
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodGet, "http://api.protect-infrastructure.apimanagement.docker.localhost/weather", nil)
	s.Require().NoError(err)
	req.Header.Add("Authorization", "Bearer "+externalToken)
	err = try.RequestWithTransport(req, 500*time.Millisecond, s.tr,
		try.StatusCodeIs(http.StatusOK),
		try.HasHeaderValue("X-Quota-Remaining", "498", true),
		try.HasHeaderValue("X-Ratelimit-Remaining", "0", true),
	)
	s.Require().NoError(err)
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

func (s *APIManagementTestSuite) checkWithBearer(method, url string, body io.Reader, bearer string, timeout time.Duration, status int) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+bearer)
	return try.RequestWithTransport(req, timeout, s.tr, try.StatusCodeIs(status))
}
