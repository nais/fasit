package naisd

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConsoleManager_handler(t *testing.T) {
	tests := []struct {
		name               string
		typ                message.ConsoleType
		data               string
		expectedNamespaces []string
		wantErr            error
	}{
		{
			name:               "create namespace test-namespace",
			typ:                message.ConsoleTypeCreateNamespace,
			data:               `{"name":"test-namespace","gcpProject":"test-project"}`,
			expectedNamespaces: []string{"test-namespace"},
		},
		{
			name:               "create namespace test-namespace again",
			typ:                message.ConsoleTypeCreateNamespace,
			data:               `{"name":"test-namespace","gcpProject":"test-project"}`,
			expectedNamespaces: []string{"test-namespace"},
		},
		{
			name:               "create namespace other-namespace",
			typ:                message.ConsoleTypeCreateNamespace,
			data:               `{"name":"other-namespace","gcpProject":"other-project"}`,
			expectedNamespaces: []string{"other-namespace", "test-namespace"},
		},
		{
			name:               "delete namespace other-namespace",
			typ:                message.ConsoleTypeDeleteNamespace,
			data:               `{"name":"other-namespace"}`,
			expectedNamespaces: []string{"test-namespace"},
		},
		{
			name:               "delete namespace nais-system",
			typ:                message.ConsoleTypeDeleteNamespace,
			data:               `{"name":"nais-system"}`,
			expectedNamespaces: []string{"test-namespace"},
			wantErr:            ErrDeleteRequiredNamespace,
		},
		{
			name:               "delete non-existing namespace",
			typ:                message.ConsoleTypeDeleteNamespace,
			data:               `{"name":"this-doesnt-exist"}`,
			expectedNamespaces: []string{"test-namespace"},
		},
	}

	ctx := context.Background()

	cs := fake.NewSimpleClientset()
	m := &ConsoleManager{
		kubeClient: cs,
		log:        logrus.NewEntry(logrus.New()),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := m.handler(ctx, message.Console{
				Type: tt.typ,
				Data: []byte(tt.data),
			})
			if gotErr != tt.wantErr {
				t.Errorf("ConsoleManager.handler() error = %v, wantErr %v", gotErr, tt.wantErr)
			}

			// Check namespaces
			namespaces, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Errorf("error listing namespaces = %v, wantErr %v", err, tt.wantErr)
			}
			names := make([]string, len(namespaces.Items))
			for i, n := range namespaces.Items {
				names[i] = n.Name
			}
			if !cmp.Equal(tt.expectedNamespaces, names) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.expectedNamespaces, names))
			}

			// Check service accounts
			for _, namespace := range tt.expectedNamespaces {
				_, err = cs.CoreV1().ServiceAccounts(namespace).Get(ctx, fmt.Sprintf("serviceuser-%s", namespace), metav1.GetOptions{})
				if err != nil {
					t.Errorf("error getting service account = %v, wantErr %v", err, tt.wantErr)
				}
				_, err = cs.RbacV1().RoleBindings(namespace).Get(ctx, fmt.Sprintf("serviceuser-%s-naisdeveloper", namespace), metav1.GetOptions{})
				if err != nil {
					t.Errorf("error getting role binding = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
