package testhelpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/envsubst"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"
)

const (
	rancherImage     = "docker.io/rancher/k3s:v1.28.8-k3s1"
	traefikNamespace = "traefik"
)

func CreateKubernetesCluster(ctx context.Context, t *testing.T) (*k3s.K3sContainer, error) {
	// Give up to three minutes to create this k8s cluster
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(3*time.Minute))
	defer cancel()
	k3sContainer, err := k3s.RunContainer(ctx,
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        rancherImage,
				ExposedPorts: []string{"80/tcp", "443/tcp", "6443/tcp", "8443/tcp"},
				NetworkMode:  "host",
				Privileged:   true,
			},
		}),
	)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if err := k3sContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})
	return k3sContainer, nil
}

// LaunchHelmCommand execute `helm` CLI with arg and display stdout+stder with testcontainer logger
func LaunchHelmCommand(t *testing.T, arg ...string) {
	logger := testcontainers.Logger

	cmd := exec.Command("helm", arg...)
	output, err := cmd.CombinedOutput()
	logger.Printf("⚙️ %s\n%s", cmd.String(), strings.TrimSpace(string(output)))
	require.NoError(t, err)
}

// LaunchKubectl execute `kubectl` CLI with arg and return stdout+stderr in a single string
func LaunchKubectl(t *testing.T, arg ...string) *bytes.Buffer {
	cmd := exec.Command("kubectl", arg...)
	output, err := cmd.Output()
	require.NoError(t, err)
	return bytes.NewBuffer(output)
}

// InstallTraefikProxy install Traefik Proxy with Helm and return LB IP
func InstallTraefikProxy(ctx context.Context, t *testing.T, k8s client.Client) (string, error) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: traefikNamespace}}
	err := k8s.Create(ctx, ns)
	assert.NoError(t, err)

	LaunchHelmCommand(t, "install", "traefik", "-n", traefikNamespace, "--wait",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)'",
		"--set", "ingressRoute.dashboard.entryPoints={web}",
		"--set", "ports.web.nodePort=30000",
		"--set", "ports.websecure.nodePort=30001",
		"traefik/traefik")

	svc := &corev1.Service{}
	err = k8s.Get(ctx, client.ObjectKey{Namespace: "traefik", Name: "traefik"}, svc)
	assert.NoError(t, err)

	return svc.Status.LoadBalancer.Ingress[0].IP, nil
}

// InstallTraefikHubAPIGW install Traefik Hub with Helm and return LB IP
func InstallTraefikHubAPIGW(ctx context.Context, t *testing.T, k8s client.Client) (string, error) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: traefikNamespace}}
	err := k8s.Create(ctx, ns)
	assert.NoError(t, err)

	CreateSecretForTraefikHub(ctx, t, k8s)
	LaunchHelmCommand(t, "install", "traefik", "-n", traefikNamespace, "--wait",
		"--set", "hub.token=license",
		"--set", "hub.platformUrl=https://platform-preview.hub.traefik.io/agent",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)'",
		"--set", "ingressRoute.dashboard.entryPoints={web}",
		"--set", "image.registry=ghcr.io",
		"--set", "image.repository=traefik/traefik-hub",
		"--set", "image.tag=v3.0.0",
		"--set", "image.pullPolicy=Always",
		"--set", "ports.web.nodePort=30000",
		"--set", "ports.websecure.nodePort=30001",
		"traefik/traefik")

	svc := &corev1.Service{}
	err = k8s.Get(ctx, client.ObjectKey{Namespace: "traefik", Name: "traefik"}, svc)
	assert.NoError(t, err)

	return svc.Status.LoadBalancer.Ingress[0].IP, nil
}

// InstallTraefikHubAPIM install Traefik Hub API Management with Helm and return LB IP
func InstallTraefikHubAPIM(ctx context.Context, t *testing.T, k8s client.Client) (string, error) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: traefikNamespace}}
	err := k8s.Create(ctx, ns)
	assert.NoError(t, err)

	CreateSecretForTraefikHub(ctx, t, k8s)
	LaunchHelmCommand(t, "install", "traefik", "-n", traefikNamespace, "--wait",
		"--set", "hub.token=license",
		"--set", "hub.platformUrl=https://platform-preview.hub.traefik.io/agent",
		"--set", "hub.apimanagement.enabled=true",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)'",
		"--set", "ingressRoute.dashboard.entryPoints={web}",
		"--set", "image.registry=ghcr.io",
		"--set", "image.repository=traefik/traefik-hub",
		"--set", "image.tag=v3.0.0",
		"--set", "image.pullPolicy=Always",
		"--set", "ports.web.nodePort=30000",
		"--set", "ports.websecure.nodePort=30001",
		"traefik/traefik")

	svc := &corev1.Service{}
	err = k8s.Get(ctx, client.ObjectKey{Namespace: "traefik", Name: "traefik"}, svc)
	assert.NoError(t, err)

	return svc.Status.LoadBalancer.Ingress[0].IP, nil
}

func CreateSecretForTraefikHub(ctx context.Context, t *testing.T, k8s client.Client) {
	apiToken, platformURL := os.Getenv("API_TOKEN"), os.Getenv("PLATFORM_URL")
	token := createGateway(t, ctx, apiToken, platformURL)

	licenseData := map[string][]byte{"token": []byte(token)}
	license := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "license", Namespace: "traefik"},
		Data:       licenseData,
	}
	err := k8s.Create(ctx, license)
	require.NoError(t, err)
}

func WaitFor(ctx context.Context, t *testing.T, k8s client.Client, interval time.Duration, labelSelector string) error {
	return wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		state := getPodState(ctx, t, k8s, labelSelector)
		if state.Terminated != nil {
			return false, fmt.Errorf("pod with label %s terminated: %v", labelSelector, state.Terminated)
		}
		return state.Running != nil, nil
	})
}

func Delete(ctx context.Context, k8s client.Client, kind, name, ns, group, version string) error {
	u := &unstructured.Unstructured{}
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Kind:    kind,
		Version: version,
	})
	testcontainers.Logger.Printf("🗑️ Deleting %s %s in %s [%s/%s]\n", kind, name, ns, group, version)
	return k8s.Delete(ctx, u)
}

func getPodState(ctx context.Context, t *testing.T, k8s client.Client, labelSelector string) corev1.ContainerState {
	podList := &corev1.PodList{}

	selector, err := labels.Parse(labelSelector)
	require.NoError(t, err)
	err = k8s.List(ctx, podList, &client.ListOptions{LabelSelector: selector})
	require.NoError(t, err)

	if len(podList.Items) != 1 {
		log.Fatalf("There should be only one pod with label %s, found %d\n", labelSelector, len(podList.Items))
	}
	status := podList.Items[0].Status
	if len(status.ContainerStatuses) != 1 {
		log.Fatalf("There should be only one container on pod labeled %s, found %d\n", labelSelector, len(status.ContainerStatuses))
	}

	return status.ContainerStatuses[0].State
}

// Inspired by Gateway API implementation
// See https://github.com/kubernetes-sigs/gateway-api/blob/main/conformance/utils/kubernetes/apply.go
// ApplyFile apply a yaml on k8s client
func ApplyFile(ctx context.Context, k8s client.Client, filepath string) (result []string, err error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(data), 4096)
	return applyYAMLOrJSONDecoder(ctx, k8s, decoder)
}

// ApplyFile execute ensubst and apply a yaml on k8s client
func ApplyEnvSubstFile(ctx context.Context, k8s client.Client, filepath string, debug bool) (result []string, err error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	output, err := envsubst.EvalEnv(string(data), false)
	if err != nil {
		return nil, err
	}
	if debug {
		testcontainers.Logger.Printf("Yaml after envsubst: %s\n", output)
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(output), 4096)
	return applyYAMLOrJSONDecoder(ctx, k8s, decoder)
}

func applyYAMLOrJSONDecoder(ctx context.Context, k8s client.Client, decoder *yaml.YAMLOrJSONDecoder) (result []string, err error) {
	resources, err := prepareResources(decoder)
	if err != nil {
		return nil, err
	}

	for i := range resources {
		uObj := &resources[i]

		namespacedName := types.NamespacedName{Namespace: uObj.GetNamespace(), Name: uObj.GetName()}
		fetchedObj := uObj.DeepCopy()
		err := k8s.Get(ctx, namespacedName, fetchedObj)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			err = k8s.Create(ctx, uObj)
			if err != nil {
				return nil, err
			}
			result = append(result, uObj.GetKind()+"/"+uObj.GetName()+" created")
			continue
		}
		uObj.SetResourceVersion(fetchedObj.GetResourceVersion())
		err = k8s.Update(ctx, uObj)
		if err != nil {
			return nil, err
		}
		result = append(result, uObj.GetKind()+"/"+uObj.GetName()+" updated")
	}
	return result, nil
}

type createGatewayResp struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	// No interest in other fields for this use case
}

// Use online API to get the Token by creating (and deleting during cleanup) the gateway
func createGateway(t *testing.T, ctx context.Context, apiToken string, platformURL string) string {
	httpClient := &http.Client{}
	clusterName := "test-" + time.Now().Format("20060102150405")
	data := bytes.NewBufferString(fmt.Sprintf(`{"name": "%s", "gatewayEndpoint": ""}`, clusterName))

	req, err := http.NewRequestWithContext(ctx, "POST", platformURL+"/cluster/external/clusters", data)
	require.NoError(t, err)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+apiToken)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)

	cgr := createGatewayResp{}
	err = json.NewDecoder(resp.Body).Decode(&cgr)
	require.NoError(t, err)

	t.Cleanup(func() {
		req, err := http.NewRequestWithContext(ctx, "DELETE", platformURL+"/cluster/external/clusters/"+cgr.ID, nil)
		if err != nil {
			t.Fatalf("failed to delete gateway on Hub Platform: %s", err)
		}
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+apiToken)
		_, err = httpClient.Do(req)
		require.NoError(t, err)
	})

	return cgr.Token
}

func prepareResources(decoder *yaml.YAMLOrJSONDecoder) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	for {
		uObj := unstructured.Unstructured{}
		if err := decoder.Decode(&uObj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(uObj.Object) == 0 {
			continue
		}

		resources = append(resources, uObj)
	}

	return resources, nil
}
