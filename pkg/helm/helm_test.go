package helm

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

func TestHelm(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/thomas/.kube/config")
	if err != nil {
		t.Fatal(err)
	}
	c := New(config, "nais-system", logrus.New().WithField("test", "helm"))

	releases, err := c.List()
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range releases {
		fmt.Println(r.Name)
	}
}
