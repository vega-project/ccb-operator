package pipelines

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	"github.com/vega-project/ccb-operator/pkg/util"
)

const (
	teffVar              = "Teff"
	logGVar              = "LogG"
	fort95Filename       = "fort.95"
	fort8Filename        = "fort.8"
	kuruzInputFilename   = "t10000_400_72.mod.7011870916"
	synspecInputFilename = "input_tlusty_fortfive"
	modFilePrefix        = "t10000_400_72_strat.mod"
)

var (
	VegaCalculationSteps = []v1.Step{
		{
			Command: "atlas12_ada",
			Args:    []string{"s"},
		},
		{
			Command: "atlas12_ada",
			Args:    []string{"r"},
		},
		{
			Command: "/bin/bash",
			Args:    []string{"-c", "synspec49 < input_tlusty_fortfive"},
		},
	}
)

type VegaPipeline struct {
	CalcPath                 string
	CalcName                 string
	AtlasControlFiles        string
	AtlasDataFiles           string
	KuruzModelTemplateFile   string
	SynspecInputTemplateFile string
	Params                   v1.Params
}

func (v *VegaPipeline) Run(logger *logrus.Entry, stepUpdaterChan chan util.Result) error {
	for index, step := range VegaCalculationSteps {
		// If there is already a status we should continue to the next step.
		// We assume that the calculation was interrupted by another process and continues now.
		if len(step.Status) != 0 {
			continue
		}

		if index == 0 {
			if err := v.GenerateKuruzInputFile(logger); err != nil {
				return fmt.Errorf("couldn't generate kuruz input file: %w", err)
			}
		}

		if index == 2 {
			if err := v.ReconstructSynspecInputFile(logger); err != nil {
				return fmt.Errorf("couldn't generate the Synspec's input file: %w", err)
			}

			if err := v.GenerateSynspecInputRuntimeFile(logger); err != nil {
				return fmt.Errorf("couldn't generate the Synspec's Runtime input file: %w", err)
			}

		}

		var status v1.CalculationPhase
		var cmdErr error
		status = v1.CompletedPhase

		// Default to 45 minutes timeout
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, step.Command, step.Args...)
		cmd.Dir = v.CalcPath

		logger = logrus.WithFields(logrus.Fields{"command": cmd.Args, "step": index})
		logger.Info("Running command and waiting for it to finish...")

		combinedOut, err := cmd.CombinedOutput()
		if err != nil {
			logger.WithError(err).WithField("output", string(combinedOut)).Error("command failed...")
			status = v1.FailedPhase
			cmdErr = err
			if err := dumpCommandOutput(logger, v.CalcPath, index, combinedOut); err != nil {
				// Debugging purposes. We don't want to exit here.
				logger.WithError(err).Error("couldn't dump command output to file")
			}
		}

		result := util.Result{
			CalcName:     v.CalcName,
			Step:         index,
			StdoutStderr: string(combinedOut),
			Status:       status,
			CommandError: cmdErr,
		}

		logger.WithField("status", status).Info("Command finished")
		stepUpdaterChan <- result

		if status == v1.FailedPhase {
			return errors.New("one or more steps failed")
		}

	}

	return nil
}

func NewVegaPipeline(calcName, calcPath string, params v1.Params) *VegaPipeline {
	return &VegaPipeline{
		AtlasControlFiles:        "atlas-control-files",
		AtlasDataFiles:           "atlas-data-files",
		KuruzModelTemplateFile:   "kuruz-model-template-file",
		SynspecInputTemplateFile: "synspec-input-template-file",
		Params:                   params,
		CalcName:                 calcName,
		CalcPath:                 calcPath,
	}
}

// GenerateInputFile generates the input file to be used by Atlas12
func (v *VegaPipeline) GenerateKuruzInputFile(logger *logrus.Entry) error {
	logger.Info("Generate Kuruz input file...")

	templateFile := filepath.Join(v.CalcPath, v.KuruzModelTemplateFile)

	data, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("could not read file %q: %v", templateFile, err)
	}

	vars := make(map[string]interface{})
	vars[teffVar] = fmt.Sprintf("%.1f", v.Params.Teff)
	vars[logGVar] = fmt.Sprintf("%.2f", v.Params.LogG)

	contents, err := parseTemplate(data, vars)
	if err != nil {
		return err
	}

	outFile := filepath.Join(v.CalcPath, kuruzInputFilename)
	logger.WithField("filename", outFile).Info("Generating input file...")
	if err := os.WriteFile(outFile, contents, 0777); err != nil {
		return fmt.Errorf("couldn't generate the new input file: %v", err)
	}

	return nil
}

// ReconstructSynspecInputFile reconstructs the synspec input file
func (v *VegaPipeline) ReconstructSynspecInputFile(logger *logrus.Entry) error {
	var contents string
	space := regexp.MustCompile(`\s+`)
	logger.Info("Reconstruct synspec input file...")

	err := filepath.Walk(v.CalcPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), modFilePrefix) {
			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				var toAppend string

				toAppend = space.ReplaceAllString(line, " ")
				if strings.HasPrefix(toAppend, "TEFF") {
					toAppend = recreateVarsLine(strings.Split(toAppend, " "))
				} else if strings.HasPrefix(toAppend, "READ DECK6 72") {
					toAppend = strings.Replace(toAppend, "READ DECK6 72", "READ DECK6 64", -1)
				}
				contents += toAppend + "\n"
			}

			if err := scanner.Err(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while walking to path")
	}

	if err := os.WriteFile(filepath.Join(v.CalcPath, "fort.8"), []byte(contents), 0777); err != nil {
		return fmt.Errorf("couldn't generate the new input file: %w", err)
	}

	return nil
}

// GenerateInputFile generates the input file to be used by sunspec.
func (v *VegaPipeline) GenerateSynspecInputRuntimeFile(logger *logrus.Entry) error {
	logger.Info("Generate synspec input file from template...")

	template := filepath.Join(v.CalcPath, v.SynspecInputTemplateFile)
	fort95File := filepath.Join(v.CalcPath, fort95Filename)

	synspecInputFile := filepath.Join(v.CalcPath, synspecInputFilename)
	data, err := os.ReadFile(template)
	if err != nil {
		return err
	}

	vars := make(map[string]interface{})
	vars[teffVar] = fmt.Sprintf("%.4f", v.Params.Teff)
	vars[logGVar] = fmt.Sprintf("%.4f", v.Params.LogG)

	contents, err := parseTemplate(data, vars)
	if err != nil {
		return err
	}

	if err := os.WriteFile(synspecInputFile, contents, 0777); err != nil {
		return fmt.Errorf("couldn't generate the new input file: %v", err)
	}

	if err := os.WriteFile(fort95File, contents, 0777); err != nil {
		return fmt.Errorf("couldn't generate the new input file: %v", err)
	}

	return nil
}

// In order to make synspec49 to be able to read the file, we need to do this ugly hack.
func recreateVarsLine(lineValues []string) string {
	// Return line for Synspec format
	if len(lineValues[1]) == 6 {
		// 2 space, 1 space, 2 spaces, 3 spaces
		return fmt.Sprintf("%s  %s %s  %s   %s",
			lineValues[0],
			lineValues[1],
			lineValues[2],
			lineValues[3],
			lineValues[4])
	}
	// 1 space, 1 space, 2 spaces, 3 spaces
	return fmt.Sprintf("%s %s %s  %s   %s",
		lineValues[0],
		lineValues[1],
		lineValues[2],
		lineValues[3],
		lineValues[4])
}

func parseTemplate(data []byte, vars interface{}) ([]byte, error) {
	var tmplBytes bytes.Buffer

	tmpl, err := template.New("tmpl").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("error while parsing the template's data: %v", err)
	}

	if err := tmpl.Execute(&tmplBytes, vars); err != nil {
		return nil, fmt.Errorf("error while executing the template: %v", err)
	}

	return tmplBytes.Bytes(), nil
}

func dumpCommandOutput(logger *logrus.Entry, calcPath string, step int, data []byte) error {
	outFile := filepath.Join(calcPath, fmt.Sprintf("step-%d", step))
	logger.WithField("filename", outFile).WithField("path", calcPath).Info("Dumping command output to a file")
	if err := os.WriteFile(outFile, data, 0777); err != nil {
		return fmt.Errorf("couldn't generate the command output file: %v", err)
	}
	return nil
}
