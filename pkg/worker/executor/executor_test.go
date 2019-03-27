package executor

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
)

const (
	testFilesFolder = "test_files"
	lessThan10kFile = "less_than_10k_example.12345"
	moreThan10kFile = "more_than_10k_example.12345"
)

func TestGenerateSynspecInputFile(t *testing.T) {
	lessThan10kExpected, moreThan10kExpected, err := loadKuruzExpectedFiles()
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		id        string
		kuruzFile string
		expected  []byte
	}{
		{
			id:        "Less than 10000 Teff",
			kuruzFile: lessThan10kFile,
			expected:  lessThan10kExpected,
		},
		{
			id:        "More than 10000 Teff",
			kuruzFile: moreThan10kFile,
			expected:  moreThan10kExpected,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			contents, err := generateSynspecInputFile(testFilesFolder, tc.kuruzFile, "fort.8")
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(contents, tc.expected) {
				t.Errorf(diff.ObjectReflectDiff(string(contents), string(tc.expected)))
			}
		})
	}
}

func loadKuruzExpectedFiles() ([]byte, []byte, error) {
	lessThan10kPath := filepath.Join(testFilesFolder, "less_than_10k_expected")
	moreThan10kPath := filepath.Join(testFilesFolder, "more_than_10k_expected")
	lessThan10k, err := ioutil.ReadFile(lessThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	moreThan10k, err := ioutil.ReadFile(moreThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	return lessThan10k, moreThan10k, nil
}

// func TestExecutorRun(t *testing.T) {

// 	nfsPath, _ := ioutil.TempDir("", "")

// 	atlasControlFiles := filepath.Join(nfsPath, "atlas_control_files/")
// 	atlasDataFiles := filepath.Join(nfsPath, "atlas_data_files/")

// 	kuruzModelTemplateFile := "kuruzModelTemplateFile"
// 	synspecInputTemplateFile := "synspecInputTemplateFile"

// 	executeChan := make(chan *calculationsv1.Calculation)
// 	stepUpdaterChan := make(chan Result)

// 	testCalc := &calculationsv1.Calculation{
// 		ObjectMeta: metav1.ObjectMeta{Name: "test"},
// 	}

// 	fakecs := fake.NewSimpleClientset(testCalc)

// 	fakecs.Fake.PrependReactor("create", "calculations", func(action clientgo_testing.Action) (bool, runtime.Object, error) {
// 		createAction := action.(clientgo_testing.CreateAction)
// 		calc := createAction.GetObject().(*calculationsv1.Calculation)

// 		fmt.Printf("Created Calc: %#v\n", calc)

// 		return false, nil, nil
// 	})

// 	fakecs.Fake.PrependReactor("update", "calculations", func(action clientgo_testing.Action) (bool, runtime.Object, error) {
// 		createAction := action.(clientgo_testing.CreateAction)
// 		calc := createAction.GetObject().(*calculationsv1.Calculation)

// 		fmt.Printf("Updated Calc: %#v\n", calc)

// 		return false, nil, nil
// 	})

// 	var wg sync.WaitGroup

// 	executor := NewExecutor(executeChan, stepUpdaterChan, nfsPath, atlasControlFiles, atlasDataFiles, kuruzModelTemplateFile, synspecInputTemplateFile)
// 	wg.Add(1)
// 	go executor.Run()

// 	calcClient := fakecs.CalculationsV1().Calculations()

// 	if _, err := calcClient.Create(testCalc); err != nil {
// 		t.Fatal(err)
// 	}

// 	executeChan <- testCalc

// 	wg.Wait()

// 	if _, err := calcClient.Get(testCalc.Name, metav1.GetOptions{}); err != nil {
// 		t.Fatal(err)
// 	}
// 	 else {
// 		fmt.Printf("CALC %#v\n", calc)
// 	}

// }
