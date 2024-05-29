package pipelines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

const (
	testFilesFolder = "testdata"
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
				return os.WriteFile(filepath.Join(tmpDir, modFilePrefix), lessThan10k, 0777)
			},
			expected: lessThan10kExpected,
		},
		{
			name: "More than 10000 Teff",
			inputFiles: func(tmpDir string) error {
				return os.WriteFile(filepath.Join(tmpDir, modFilePrefix), moreThan10k, 0777)
			},
			expected: moreThan10kExpected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dir, err := os.MkdirTemp("", "")
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
	lessThan10k, err := os.ReadFile(lessThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	moreThan10k, err := os.ReadFile(moreThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	return lessThan10k, moreThan10k, nil
}

func loadKuruzInputFiles() ([]byte, []byte, error) {
	lessThan10kPath := filepath.Join(testFilesFolder, "less_than_10k_example.12345")
	moreThan10kPath := filepath.Join(testFilesFolder, "more_than_10k_example.12345")
	lessThan10k, err := os.ReadFile(lessThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	moreThan10k, err := os.ReadFile(moreThan10kPath)
	if err != nil {
		return nil, nil, err
	}

	return lessThan10k, moreThan10k, nil
}

func Test_constructKuruzInputFileLine(t *testing.T) {
	tests := []struct {
		name string
		teff int
		logg float64
		want string
	}{
		{
			name: "Less than 10k Teff",
			teff: 9000,
			logg: 4,
			want: "TEFF   9000.  GRAVITY 4.00000 LTE ",
		},
		{
			name: "More than 10k Teff",
			teff: 10000,
			logg: 4,
			want: "TEFF  10000.  GRAVITY 4.00000 LTE ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(constructKuruzInputFileLine(tt.teff, tt.logg), tt.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
