package scheduler

import (
	"context"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestScheduler_Run(t *testing.T) {

	tests := []struct {
		name             string
		calc             v1.Calculation
		initialResources []ctrlruntimeclient.Object
		expected         []ctrlruntimeclient.Object
	}{
		{
			name: "basic case",
			calc: v1.Calculation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-calc",
					Namespace: "vega",
				},
				WorkerPool: "workerpool-test",
			},
			initialResources: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerAvailableState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
			},
			expected: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerAvailableState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerAvailableState},
						},
					},
				},
				&v1.Calculation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-calc",
						Namespace: "vega",
						Labels:    map[string]string{"vegaproject.io/assign": "worker-1"},
					},
					Assign:     "worker-1",
					WorkerPool: "workerpool-test",
				},
			},
		},

		{
			name: "basic case - no workers available",
			calc: v1.Calculation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-calc",
					Namespace: "vega",
				},
				WorkerPool: "workerpool-test",
			},
			initialResources: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerReservedState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerReservedState},
						},
					},
				},
			},
			expected: []ctrlruntimeclient.Object{
				&workersv1.WorkerPool{
					ObjectMeta: metav1.ObjectMeta{Namespace: "vega", Name: "workerpool-test"},
					Spec: workersv1.WorkerPoolSpec{
						Workers: map[string]workersv1.Worker{
							"node-1": {Name: "worker-1", Node: "node-1", State: workersv1.WorkerReservedState},
							"node-2": {Name: "worker-2", Node: "node-2", State: workersv1.WorkerReservedState},
							"node-3": {Name: "worker-3", Node: "node-3", State: workersv1.WorkerReservedState},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculationCh := make(chan v1.Calculation)
			stopCh := make(chan struct{})
			wg := &sync.WaitGroup{}
			client := fakectrlruntimeclient.NewClientBuilder().WithObjects(tt.initialResources...).Build()
			s := &Scheduler{
				logger:        logrus.WithField("name", tt.name),
				calculationCh: calculationCh,
				client:        client,
			}

			wg.Add(1)
			go s.Run(context.Background(), stopCh, wg)

			calculationCh <- tt.calc
			stopCh <- struct{}{}
			wg.Wait()
			var actualObjects []ctrlruntimeclient.Object

			var actualWorkerPool workersv1.WorkerPool
			if err := client.Get(context.Background(), ctrlruntimeclient.ObjectKey{Namespace: "vega", Name: "workerpool-test"}, &actualWorkerPool); err != nil {
				t.Fatal(err)
			}

			actualObjects = append(actualObjects, &actualWorkerPool)

			var actualCalculations v1.CalculationList
			if err := client.List(context.Background(), &actualCalculations); err != nil {
				t.Fatal(err)
			}

			for _, item := range actualCalculations.Items {
				actualObjects = append(actualObjects, &item)
			}

			if diff := cmp.Diff(actualObjects, tt.expected,
				cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion"),
				cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
				cmpopts.IgnoreFields(workersv1.Worker{}, "LastUpdateTime")); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
