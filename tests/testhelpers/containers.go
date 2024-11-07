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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	batchv1 "k8s.io/api/batch/v1"
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
	rancherImage     = "docker.io/rancher/k3s:v1.30.5-k3s1"
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
				HostConfigModifier: func(hc *container.HostConfig) {
					hc.NetworkMode = network.NetworkBridge
					hc.Privileged = true
				},
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
		"--version", "v33.0.0",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.enabled=true",
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
		"--version", "v33.0.0",
		"--set", "hub.token=traefik-hub-license",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.enabled=true",
		"--set", "ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)'",
		"--set", "ingressRoute.dashboard.entryPoints={web}",
		"--set", "image.registry=ghcr.io",
		"--set", "image.repository=traefik/traefik-hub",
		"--set", "image.tag=v3.6.0",
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
		"--version", "v33.0.0",
		"--set", "hub.token=traefik-hub-license",
		"--set", "hub.apimanagement.enabled=true",
		"--set", "ingressClass.enabled=false",
		"--set", "ingressRoute.dashboard.enabled=true",
		"--set", "ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)'",
		"--set", "ingressRoute.dashboard.entryPoints={web}",
		"--set", "image.registry=ghcr.io",
		"--set", "image.repository=traefik/traefik-hub",
		"--set", "image.tag=v3.6.0",
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
		ObjectMeta: metav1.ObjectMeta{Name: "traefik-hub-license", Namespace: "traefik"},
		Data:       licenseData,
	}
	err := k8s.Create(ctx, license)
	require.NoError(t, err)
}

func WaitForPodReady(ctx context.Context, t *testing.T, k8s client.Client, interval time.Duration, labelSelector string) error {
	ctx, cancelFunc := context.WithTimeout(ctx, interval)

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		state := getPodState(ctx, t, k8s, labelSelector)
		if state.Terminated != nil {
			cancelFunc()
			return false, fmt.Errorf("pod with label %s terminated: %v", labelSelector, state.Terminated)
		}

		if state.Running != nil {
			cancelFunc()
			return true, nil
		}

		return false, nil
	})
}

func WaitForJobCompleted(ctx context.Context, t *testing.T, k8s client.Client, interval time.Duration, labelSelector string) error {
	ctx, cancelFunc := context.WithTimeout(ctx, interval)

	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		state := getJobState(ctx, t, k8s, labelSelector)
		if state.Failed > 0 {
			cancelFunc()
			return false, fmt.Errorf("job with label %s terminated: %v", labelSelector, state.Conditions[0].Message)
		}

		if state.Succeeded > 0 {
			cancelFunc()
			return true, nil
		}

		return false, nil
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

func getJobState(ctx context.Context, t *testing.T, k8s client.Client, labelSelector string) batchv1.JobStatus {
	jobList := &batchv1.JobList{}

	selector, err := labels.Parse(labelSelector)
	require.NoError(t, err)
	err = k8s.List(ctx, jobList, &client.ListOptions{LabelSelector: selector})
	require.NoError(t, err)

	if len(jobList.Items) != 1 {
		log.Fatalf("There should be only one job with label %s, found %d\n", labelSelector, len(jobList.Items))
	}

	return jobList.Items[0].Status
}

// Inspired by Gateway API implementation
// See https://github.com/kubernetes-sigs/gateway-api/blob/main/conformance/utils/kubernetes/apply.go

// ApplyFile applies a yaml on k8s client
func ApplyFile(ctx context.Context, k8s client.Client, filepath string) (result []string, err error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(data), 4096)
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

	// fail fast if API TOKEN is invalid
	require.NotEmpty(t, cgr.Token)

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
