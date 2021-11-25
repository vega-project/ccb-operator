package calculations

import (
	"fmt"

	"k8s.io/client-go/kubernetes/scheme"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
)

func init() {
	if err := v1.AddToScheme(scheme.Scheme); err != nil {
		panic(fmt.Sprintf("failed to register imagev1 scheme: %v", err))
	}
}
