package testhelpers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kubeConfigEnvVar = "KUBECONFIG"

func apply(t *testing.T, ctx context.Context, k8s client.Client, path string) {
	t.Helper()

	results, err := ApplyFile(ctx, k8s, filepath.Join("..", "..", path))
	require.NoError(t, err)
	testcontainers.Logger.Printf("📦️ %q loaded\n", results)
}

func Test_WaitForPodReady(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	k3d, err := CreateKubernetesCluster(ctx, t)
	require.NoError(t, err)

	kubeConfigYaml, err := k3d.GetKubeConfig(ctx)
	require.NoError(t, err)

	f, err := os.CreateTemp(t.TempDir(), "kbcfg-")
	require.NoError(t, err)
	defer f.Close()

	_, err = f.Write(kubeConfigYaml)
	require.NoError(t, err)

	t.Setenv(kubeConfigEnvVar, f.Name())
	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	require.NoError(t, err)

	k8s, err := client.New(restcfg, client.Options{})
	require.NoError(t, err)

	apply(t, ctx, k8s, "src/manifests/apps-namespace.yaml")
	apply(t, ctx, k8s, "src/manifests/whoami-app.yaml")

	// whoami takes around 30s to be deployed to it should be enough for WaitForPodReady to fail in the context of this test
	timeout := 90

	go func() {
		podRunning := false
		for i := 0; i < timeout; i++ {
			time.Sleep(time.Second)
			state := getPodState(ctx, t, k8s, "app=whoami")
			if state.Running != nil {
				podRunning = true
				break
			}
		}

		if podRunning {
			time.Sleep(time.Second)
			t.Log("whoami is running !")
			cancelFunc()
		}
	}()

	time.Sleep(1 * time.Second)

	err = WaitForPodReady(ctx, t, k8s, time.Duration(timeout)*time.Second, "app=whoami")
	require.NoError(t, err)
}
