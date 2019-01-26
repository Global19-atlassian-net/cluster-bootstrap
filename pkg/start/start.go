package start

import (
	"fmt"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const assetTimeout = 20 * time.Minute

type Config struct {
	AssetDir        string
	PodManifestPath string
	Strict          bool
	RequiredPods    []string
}

type startCommand struct {
	podManifestPath string
	assetDir        string
	strict          bool
	requiredPods    []string
}

func NewStartCommand(config Config) (*startCommand, error) {
	return &startCommand{
		assetDir:        config.AssetDir,
		podManifestPath: config.PodManifestPath,
		strict:          config.Strict,
		requiredPods:    config.RequiredPods,
	}, nil
}

func (b *startCommand) Run() error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", filepath.Join(b.assetDir, assetPathAdminKubeConfig))
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	bcp := newBootstrapControlPlane(b.assetDir, b.podManifestPath)

	defer func() {
		// Always tear down the bootstrap control plane and clean up manifests and secrets.
		if err := bcp.Teardown(); err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()

	defer func() {
		// Always report errors.
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()

	if err = bcp.Start(); err != nil {
		return err
	}

	if err = createAssets(restConfig, filepath.Join(b.assetDir, assetPathManifests), assetTimeout, b.strict); err != nil {
		return err
	}

	if err = waitUntilPodsRunning(client, b.requiredPods, assetTimeout); err != nil {
		return err
	}

	return nil
}

// All start command printing to stdout should go through this fmt.Printf wrapper.
// The stdout of the start command should convey information useful to a human sitting
// at a terminal watching their cluster bootstrap itself. Otherwise the message
// should go to stderr.
func UserOutput(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
