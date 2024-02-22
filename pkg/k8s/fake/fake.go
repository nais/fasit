package fake

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aquasecurity/trivy-operator/pkg/apis/aquasecurity/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

type clients struct {
	ClientSet kubernetes.Interface
	Dynamic   dynamic.Interface
}

// Clients returns a new fake kubernetes clientset for each directory at root in the given directory.
// Each yaml file in the directory will be created as a resource, where resources in a "teams" directory
// will be created in a namespace with the same name as the file.
func Clients(dir fs.FS) func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
	scheme := newScheme()

	resources := make(map[string][]runtime.Object)
	// TODO: use yaml file in the data dir on root?
	fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		cluster := parseCluster(path)
		if cluster == "" {
			return nil
		}

		resources[cluster] = append(resources[cluster], parseResources(dir, path)...)

		return nil
	})

	ret := make(map[string]clients)
	for cluster, objs := range resources {
		ret[cluster] = clients{
			ClientSet: fake.NewSimpleClientset(objs...),
			Dynamic:   dynfake.NewSimpleDynamicClient(scheme, objs...),
		}
	}

	return func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
		c, ok := ret[cluster]
		if !ok {
			return nil, nil, fmt.Errorf("no fake client for cluster %s", cluster)
		}

		return c.ClientSet, c.Dynamic, nil
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	v1alpha1.AddToScheme(scheme)
	return scheme
}

func parseCluster(path string) string {
	p := strings.SplitN(path, string(os.PathSeparator), 2)
	if len(p) == 0 {
		return ""
	}

	return p[0]
}

func parseResources(dir fs.FS, path string) []runtime.Object {
	b, err := fs.ReadFile(dir, path)
	if err != nil {
		return nil
	}

	parts := bytes.Split(b, []byte("\n---"))

	ret := make([]runtime.Object, 0, len(parts))
	for _, p := range parts {
		if len(bytes.TrimSpace(p)) == 0 {
			continue
		}

		r := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(p, &r); err != nil {
			panic(err)
		}

		ns := strings.Trim(filepath.Base(filepath.Dir(path)), string(filepath.Separator))

		r.SetNamespace(ns)
		lbls := r.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		lbls["team"] = ns
		r.SetLabels(lbls)
		ret = append(ret, r)
	}

	return ret
}
