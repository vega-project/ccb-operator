package pipelines

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

const (
	testFilesFolder = "testdata"
	lessThan10kFile = "less_than_10k_example.12345"
	moreThan10kFile = "more_than_10k_example.12345"
)

func TestVegaPipeline_ReconstructSynspecInputFile(t *testing.T) {
	lessThan10kExpected, moreThan10kExpected, err := loadKuruzExpectedFiles()
	if err != nil {
		t.Fatal(err)
	}

	lessThan10k, moreThan10k, err := loadKuruzInputFiles()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		inputFiles func(string) error
		expected   []byte
		wantErr    bool
	}{
		{
			name: "Less than 10000 Teff",
			inputFiles: func(tmpDir string) error {
				return ioutil.WriteFile(filepath.Join(tmpDir, modFilePrefix), lessThan10k, 0777)
			},
			expected: lessThan10kExpected,
		},
		{
			name: "More than 10000 Teff",
			inputFiles: func(tmpDir string) error {
				return ioutil.WriteFile(filepath.Join(tmpDir, modFilePrefix), moreThan10k, 0777)
			},
			expected: moreThan10kExpected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			if err := tt.inputFiles(dir); err != nil {
				t.Fatal(err)
			}

			v := &VegaPipeline{
				CalcPath: dir,
			}

			if err := v.ReconstructSynspecInputFile(logrus.WithField("test-name", tt.name)); (err != nil) != tt.wantErr {
				t.Errorf("VegaPipeline.ReconstructSynspecInputFile() error = %v, wantErr %v", err, tt.wantErr)
			}

			b, err := os.ReadFile(filepath.Join(dir, fort8Filename))
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(string(b), string(tt.expected)); diff != "" {
				t.Fatal(diff)
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

func loadKuruzInputFiles() ([]byte, []byte, error) {
	lessThan10kPath := filepath.Join(testFilesFolder, "less_than_10k_example.12345")
	moreThan10kPath := filepath.Join(testFilesFolder, "more_than_10k_example.12345")
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