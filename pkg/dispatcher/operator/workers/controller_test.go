package workers

import (
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
)

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

	type testData struct {
		name   string
		values map[string]interface{}
	}

	type picked struct {
		name string
		teff string
		logG string
	}

	testCases := []struct {
		id           string
		data         []testData
		expected     map[string]map[string]string
		expectToPick picked
	}{
		{
			id: "single value",
			data: []testData{
				{
					name:   "vz.star:teff_10000",
					values: map[string]interface{}{"teff": "10000.0", "logG": "4"},
				},
			},
			expected: map[string]map[string]string{
				"vz.star:teff_10000": {"teff": "10000.0", "logG": "4", "status": "Processing"},
			},
			expectToPick: picked{name: "vz.star:teff_10000", teff: "10000.0", logG: "4"},
		},

		{
			id: "multiple values",
			data: []testData{
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
			expected: map[string]map[string]string{
				"vz.star:teff_10000": {"teff": "10000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_11000": {"teff": "11000.0", "logG": "4"},
				"vz.star:teff_12000": {"teff": "12000.0", "logG": "4"},
				"vz.star:teff_13000": {"teff": "13000.0", "logG": "4"},
			},
			expectToPick: picked{name: "vz.star:teff_10000", teff: "10000.0", logG: "4"},
		},

		{
			id: "multiple values, existing processing status",
			data: []testData{
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
			expected: map[string]map[string]string{
				"vz.star:teff_10000": {"teff": "10000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_11000": {"teff": "11000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_12000": {"teff": "12000.0", "logG": "4"},
				"vz.star:teff_13000": {"teff": "13000.0", "logG": "4"},
			},
			expectToPick: picked{name: "vz.star:teff_11000", teff: "11000.0", logG: "4"},
		},
		{
			id: "multiple values, existing processing statuses",
			data: []testData{
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
			expected: map[string]map[string]string{
				"vz.star:teff_10000": {"teff": "10000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_11000": {"teff": "11000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_12000": {"teff": "12000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_13000": {"teff": "13000.0", "logG": "4", "status": "Processing"},
			},
			expectToPick: picked{name: "vz.star:teff_13000", teff: "13000.0", logG: "4"},
		},

		{
			id: "multiple values, scrambled existing processing statuses",
			data: []testData{
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
			expected: map[string]map[string]string{
				"vz.star:teff_10000": {"teff": "10000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_11000": {"teff": "11000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_12000": {"teff": "12000.0", "logG": "4", "status": "Processing"},
				"vz.star:teff_13000": {"teff": "13000.0", "logG": "4"},
			},
			expectToPick: picked{name: "vz.star:teff_11000", teff: "11000.0", logG: "4"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			controller := &Controller{
				logger:      logrus.WithField("test-name", tc.id),
				redisClient: redisClient,
			}

			var counter float64
			for _, testData := range tc.data {
				if boolCmd := redisClient.HMSet(testData.name, testData.values); boolCmd.Err() != nil {
					t.Fatalf("couldn't set test data: %v", boolCmd.Err())
				}
				if stringCmd := redisClient.ZAdd("vz", redis.Z{Score: counter, Member: testData.name}); stringCmd.Err() != nil {
					t.Fatalf("couldn't set test data: %v", stringCmd.Err())
				}
				counter = counter + 1

			}

			// Update assigned calculation
			actualName, actualTeff, actualLogG := controller.assignCalulationDB()
			actualPicked := picked{name: actualName, teff: actualTeff, logG: actualLogG}

			if !reflect.DeepEqual(actualPicked, tc.expectToPick) {
				t.Fatalf("\nexpected: %#v\ngot: %#v", tc.expectToPick, actualPicked)

			}

			for _, testData := range tc.data {
				newVal := redisClient.HGetAll(testData.name).Val()
				if !reflect.DeepEqual(newVal, tc.expected[testData.name]) {
					t.Fatalf("\nexpected: %#v\ngot: %#v", tc.expected[testData.name], newVal)
				}
			}

			redisClient.FlushDB()

		})
	}

}
