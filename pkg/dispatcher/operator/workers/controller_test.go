package workers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	calculationsv1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/client/clientset/versioned/fake"
)

func TestDeleteAssignedCalculation(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})

	type redisTestData struct {
		name   string
		values map[string]interface{}
	}

	type testData struct {
		redisData    []redisTestData
		calculations []calculationsv1.Calculation
	}
	testCases := []struct {
		id            string
		podName       string
		redisTestData []redisTestData
		calculations  []calculationsv1.Calculation
		expected      testData
		errorExpected bool
	}{
		{
			id:      "no calculation to delete, expected to throw a warning",
			podName: "test-pod-empty",
		},
		{
			id:      "one calculation to delete",
			podName: "test-pod-one",
			redisTestData: []redisTestData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
				},
			},
			calculations: []calculationsv1.Calculation{
				{
					Assign:     "test-pod-one",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-1", Labels: map[string]string{"assign": "test-pod-one"}},
					DBKey:      "vz.star:teff_10000",
					Phase:      calculationsv1.ProcessingPhase,
				},
			},
			expected: testData{
				redisData: []redisTestData{
					{
						name:   "vz.star:teff_10000",
						values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": ""},
					},
				},
			},
		},
		{
			id:      "more than one calculation to delete, error expected",
			podName: "test-pod-more-than-one",
			redisTestData: []redisTestData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_20000",
					values: map[string]interface{}{"teff": "20000.0", "logG": "4", "status": "Processing"},
				},
			},
			calculations: []calculationsv1.Calculation{
				{
					Assign:     "test-pod-more-than-one",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more", Labels: map[string]string{"assign": "test-pod-more-than-one"}},
					DBKey:      "vz.star:teff_10000",
					Phase:      calculationsv1.ProcessingPhase,
				},
				{
					Assign:     "test-pod-more-than-one",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more-1", Labels: map[string]string{"assign": "test-pod-more-than-one"}},
					DBKey:      "vz.star:teff_20000",
					Phase:      calculationsv1.ProcessingPhase,
				},
			},
			expected: testData{
				redisData: []redisTestData{
					{
						name:   "vz.star:teff_10000",
						values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
					},
					{
						name:   "vz.star:teff_20000",
						values: map[string]interface{}{"teff": "20000.0", "logG": "4", "status": "Processing"},
					},
				},
				calculations: []calculationsv1.Calculation{
					{
						Assign:     "test-pod-more-than-one",
						ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more", Labels: map[string]string{"assign": "test-pod-more-than-one"}},
						DBKey:      "vz.star:teff_10000",
						Phase:      calculationsv1.ProcessingPhase,
					},
					{
						Assign:     "test-pod-more-than-one",
						ObjectMeta: metav1.ObjectMeta{Name: "test-calc-more-1", Labels: map[string]string{"assign": "test-pod-more-than-one"}},
						DBKey:      "vz.star:teff_20000",
						Phase:      calculationsv1.ProcessingPhase,
					},
				},
			},
			errorExpected: true,
		},
		{
			id:      "more than one calculation, but only one to delete",
			podName: "test-pod-one-deletion",
			redisTestData: []redisTestData{
				{
					name:   "vz.star:teff_40000",
					values: map[string]interface{}{"teff": "40000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_30000",
					values: map[string]interface{}{"teff": "30000.0", "logG": "4", "status": "Processing"},
				},
			},
			calculations: []calculationsv1.Calculation{
				{
					Assign:     "test-pod-one-deletion",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-del", Labels: map[string]string{"assign": "test-pod-one-deletion"}},
					DBKey:      "vz.star:teff_40000",
					Phase:      calculationsv1.ProcessingPhase,
				},
				{
					Assign:     "test-pod-no-delete",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-no-del", Labels: map[string]string{"assign": "test-pod-no-delete"}},
					DBKey:      "vz.star:teff_30000",
					Phase:      calculationsv1.ProcessingPhase,
				},
			},
			expected: testData{
				redisData: []redisTestData{
					{
						name:   "vz.star:teff_40000",
						values: map[string]interface{}{"teff": "40000.0", "logG": "4", "status": ""},
					},
					{
						name:   "vz.star:teff_30000",
						values: map[string]interface{}{"teff": "30000.0", "logG": "4", "status": "Processing"},
					},
				},
				calculations: []calculationsv1.Calculation{
					{
						Assign:     "test-pod-no-delete",
						ObjectMeta: metav1.ObjectMeta{Name: "test-calc-no-del", Labels: map[string]string{"assign": "test-pod-no-delete"}},
						DBKey:      "vz.star:teff_30000",
						Phase:      calculationsv1.ProcessingPhase,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		var counter float64
		for _, testData := range tc.redisTestData {
			if boolCmd := redisClient.HMSet(testData.name, testData.values); boolCmd.Err() != nil {
				t.Fatalf("couldn't set test data: %v", boolCmd.Err())
			}
			if stringCmd := redisClient.ZAdd("vz", redis.Z{Score: counter, Member: testData.name}); stringCmd.Err() != nil {
				t.Fatalf("couldn't set test data: %v", stringCmd.Err())
			}
			counter = counter + 1
		}

		fakeCalculationClient := fake.NewSimpleClientset()

		for _, c := range tc.calculations {
			_, err := fakeCalculationClient.VegaV1().Calculations().Create(context.TODO(), &c, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}

		controller := &Controller{
			logger:            logrus.WithField("test-name", tc.id),
			redisClient:       redisClient,
			calculationClient: fakeCalculationClient,
		}

		if err := controller.deleteAssignedCalculation(tc.podName); err != nil && !tc.errorExpected {
			t.Fatalf("Something went wrong in deleteCalculation(): %v", err)
		} else if err == nil && tc.errorExpected {
			t.Fatal("We expected to get an error, but didn't get one")
		}

		actualCalculations, err := fakeCalculationClient.VegaV1().Calculations().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}

		var actualRedisData []redisTestData

		for _, redisData := range tc.redisTestData {
			calculationValues, err := redisClient.HGetAll(redisData.name).Result()
			if err != nil {
				t.Fatal(err)
			}

			calculationValuesInterface := make(map[string]interface{})
			for key, value := range calculationValues {
				calculationValuesInterface[key] = value
			}

			actualRedisData = append(actualRedisData, redisTestData{
				name:   redisData.name,
				values: calculationValuesInterface,
			})
		}

		actualTestData := testData{
			redisData:    actualRedisData,
			calculations: actualCalculations.Items,
		}

		if diff := cmp.Diff(tc.expected, actualTestData, cmp.AllowUnexported(testData{}, redisTestData{})); diff != "" {
			t.Fatal(diff)
		}
		redisClient.FlushDB()

	}
}

func TestAssignCalulationDB(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})

	type picked struct {
		name string
		teff string
		logG string
	}

	testCases := []struct {
		id                 string
		data               []testDBData
		expectToPick       picked
		redisSortedSetName string
	}{
		{
			id:                 "single value",
			redisSortedSetName: "vz",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4"},
				},
			},
			expectToPick: picked{name: "vz.star:teff_10000", teff: "10000.0", logG: "4"},
		},
		{
			id:                 "multiple values",
			redisSortedSetName: "vz",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_11000",
					values: map[string]interface{}{"teff": "11000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_12000",
					values: map[string]interface{}{"teff": "12000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_13000",
					values: map[string]interface{}{"teff": "13000.0", "logG": "4"},
				},
			},
			expectToPick: picked{name: "vz.star:teff_10000", teff: "10000.0", logG: "4"},
		},

		{
			id:                 "multiple values, existing processing status",
			redisSortedSetName: "vz",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_11000",
					values: map[string]interface{}{"teff": "11000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_12000",
					values: map[string]interface{}{"teff": "12000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_13000",
					values: map[string]interface{}{"teff": "13000.0", "logG": "4"},
				},
			},
			expectToPick: picked{name: "vz.star:teff_11000", teff: "11000.0", logG: "4"},
		},
		{
			id:                 "multiple values, existing processing statuses",
			redisSortedSetName: "xy",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_11000",
					values: map[string]interface{}{"teff": "11000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_12000",
					values: map[string]interface{}{"teff": "12000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_13000",
					values: map[string]interface{}{"teff": "13000.0", "logG": "4"},
				},
			},
			expectToPick: picked{name: "vz.star:teff_13000", teff: "13000.0", logG: "4"},
		},

		{
			id:                 "multiple values, scrambled existing processing statuses",
			redisSortedSetName: "vz-test",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_11000",
					values: map[string]interface{}{"teff": "11000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_12000",
					values: map[string]interface{}{"teff": "12000.0", "logG": "4", "status": "Processing"},
				},
				{
					name:   "vz.star:teff_13000",
					values: map[string]interface{}{"teff": "13000.0", "logG": "4"},
				},
			},
			expectToPick: picked{name: "vz.star:teff_11000", teff: "11000.0", logG: "4"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			controller := &Controller{
				logger:             logrus.WithField("test-name", tc.id),
				redisClient:        redisClient,
				redisSortedSetName: tc.redisSortedSetName,
			}

			if err := fillDB(redisClient, tc.redisSortedSetName, tc.data); err != nil {
				t.Fatal(err)
			}

			// Update assigned calculation
			actualName, actualTeff, actualLogG := controller.assignCalulationDB()
			actualPicked := picked{name: actualName, teff: actualTeff, logG: actualLogG}

			if !reflect.DeepEqual(actualPicked, tc.expectToPick) {
				t.Fatalf("\nexpected: %#v\ngot: %#v", tc.expectToPick, actualPicked)

			}
			redisClient.FlushDB()
		})
	}
}

type testDBData struct {
	name   string
	values map[string]interface{}
}

func fillDB(redisClient *redis.Client, redisSortedSetName string, testDBDataList []testDBData) error {
	var counter float64
	for _, testDBData := range testDBDataList {
		if boolCmd := redisClient.HMSet(testDBData.name, testDBData.values); boolCmd.Err() != nil {
			return fmt.Errorf("couldn't set test data: %v", boolCmd.Err())
		}
		if stringCmd := redisClient.ZAdd(redisSortedSetName, redis.Z{Score: counter, Member: testDBData.name}); stringCmd.Err() != nil {
			return fmt.Errorf("couldn't set test data: %v", stringCmd.Err())
		}
		counter = counter + 1
	}
	return nil
}

func TestCreateCalculationForPod(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
		DB:   0,
	})

	testCases := []struct {
		id                   string
		podName              string
		data                 []testDBData
		calculations         []calculationsv1.Calculation
		expectedCalculations []calculationsv1.Calculation
	}{
		{
			id:      "created by human, single happy case",
			podName: "test-pod",
			calculations: []calculationsv1.Calculation{
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
			},
			expectedCalculations: []calculationsv1.Calculation{
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
			},
		},
		{
			id:      "created by human, multiple happy case",
			podName: "test-pod",
			calculations: []calculationsv1.Calculation{
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 13000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 30, 0, 0, time.UTC)}},
				},
			},
			expectedCalculations: []calculationsv1.Calculation{
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 12000.0,
						LogG: 4.0,
					},
					Assign:     "test-pod",
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 20, 0, 0, time.UTC)}},
				},
				{
					Spec: calculationsv1.CalculationSpec{
						Teff: 13000.0,
						LogG: 4.0,
					},
					ObjectMeta: metav1.ObjectMeta{Name: "test-calc-2", Labels: map[string]string{"created_by_human": "true"}},
					Phase:      calculationsv1.CreatedPhase,
					Status:     calculationsv1.CalculationStatus{StartTime: metav1.Time{Time: time.Date(2000, 0, 0, 15, 30, 0, 0, time.UTC)}},
				},
			},
		},
		{
			id:      "happy case, create calculation from redis",
			podName: "test-pod",
			data: []testDBData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_11000",
					values: map[string]interface{}{"teff": "11000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_12000",
					values: map[string]interface{}{"teff": "12000.0", "logG": "4"},
				},
				{
					name:   "vz.star:teff_13000",
					values: map[string]interface{}{"teff": "13000.0", "logG": "4"},
				},
			},
			expectedCalculations: []calculationsv1.Calculation{
				{
					TypeMeta:   metav1.TypeMeta{Kind: "Calculation", APIVersion: "vega.io/v1"},
					ObjectMeta: metav1.ObjectMeta{Name: "calc-xc864fxvd5xccn6x", Labels: map[string]string{"assign": "test-pod"}},
					DBKey:      "vz.star:teff_10000",
					Assign:     "test-pod",
					Phase:      calculationsv1.CreatedPhase,
					Spec: calculationsv1.CalculationSpec{
						Steps: []calculationsv1.Step{
							{Command: "atlas12_ada", Args: []string{"s"}},
							{Command: "atlas12_ada", Args: []string{"r"}},
							{Command: "synspec49", Args: []string{"<", "input_tlusty_fortfive"}},
						},
						Teff: 10000,
						LogG: 4,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		fakeCalculationClient := fake.NewSimpleClientset()

		for _, c := range tc.calculations {
			_, err := fakeCalculationClient.VegaV1().Calculations().Create(context.TODO(), &c, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}

		if err := fillDB(redisClient, "vz", tc.data); err != nil {
			t.Fatal(err)
		}

		c := &Controller{
			logger:             logrus.WithField("test-name", tc.id),
			calculationClient:  fakeCalculationClient,
			redisClient:        redisClient,
			redisSortedSetName: "vz",
		}

		if err := c.createCalculationForPod(tc.podName); err != nil {
			t.Fatal(err)
		}

		actualCalculations, err := c.calculationClient.VegaV1().Calculations().List(c.ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(tc.expectedCalculations, actualCalculations.Items, cmpopts.IgnoreFields(metav1.Time{}, "Time")); diff != "" {
			t.Fatal(diff)
		}
		redisClient.FlushDB()
	}
}
