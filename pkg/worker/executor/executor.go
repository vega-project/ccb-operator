package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/vega-project/ccb-operator/pkg/apis/calculations/v1"
	workersv1 "github.com/vega-project/ccb-operator/pkg/apis/workers/v1"
	"github.com/vega-project/ccb-operator/pkg/grpc"
	"github.com/vega-project/ccb-operator/pkg/pipelines"
	"github.com/vega-project/ccb-operator/pkg/util"
)

type Executor struct {
	logger          *logrus.Entry
	executeChan     chan *v1.Calculation
	stepUpdaterChan chan util.Result
	calcErrorChan   chan string
	Status          string
	nfsPath         string
	client          ctrlruntimeclient.Client
	ctx             context.Context
	nodename        string
	namespace       string
	workerPool      string
	grpcClient      *grpc.Client
}

func NewExecutor(
	ctx context.Context,
	client ctrlruntimeclient.Client,
	executeChan chan *v1.Calculation,
	calcErrorChan chan string,
	stepUpdaterChan chan util.Result,
	nfsPath,
	nodename,
	namespace,
	workerPool string,
	grpcClient *grpc.Client) *Executor {
	return &Executor{
		ctx:             ctx,
		client:          client,
		executeChan:     executeChan,
		stepUpdaterChan: stepUpdaterChan,
		calcErrorChan:   calcErrorChan,
		nfsPath:         nfsPath,
		nodename:        nodename,
		namespace:       namespace,
		workerPool:      workerPool,
		grpcClient:      grpcClient,
	}
}

func (e *Executor) Run() {
	for {
		select {
		case calc := <-e.executeChan:
			e.logger = logrus.WithField("for-calculation", calc.Name)

			// TODO: Can this run only once when the worker is starting????????????
			// Setting stack limit
			if err := setUnlimitStack(); err != nil {
				e.logger.WithError(err).Error("couln't set stack limit")
				e.calcErrorChan <- calc.Name
				break
			}

			rootFolder := filepath.Join(e.nfsPath, calc.Labels[util.CalcRootFolder])
			calcPath, err := os.MkdirTemp("", calc.Labels[util.CalculationNameLabel])
			if err != nil {
				e.logger.WithError(err).Error("error creating temp directory")
				e.calcErrorChan <- calc.Name
				break
			}

			if _, err := os.Stat(calcPath); err != nil {
				if err := os.MkdirAll(calcPath, 0777); err != nil {
					e.logger.WithError(err).Error("couln't create directory. Aborting...")
					e.calcErrorChan <- calc.Name
					break
				}
			}

			if calc.InputFiles != nil {
				for _, inputFile := range calc.InputFiles.Files {
					if calc.InputFiles.Symlink {
						if err := e.createSymbolicLinks([]string{filepath.Join(rootFolder, inputFile)}, calcPath); err != nil {
							e.logger.WithError(err).Error("couln't creating symlink. Aborting...")
							e.calcErrorChan <- calc.Name
							break
						}
						continue
					}

					input, err := os.ReadFile(filepath.Join(rootFolder, inputFile))
					if err != nil {
						e.logger.WithError(err).Error("couln't read input file. Aborting...")
						e.calcErrorChan <- calc.Name
						return
					}

					_, inputFilename := filepath.Split(inputFile)
					destinationFile := filepath.Join(calcPath, inputFilename)
					err = os.WriteFile(destinationFile, input, 0644)
					if err != nil {
						e.logger.WithError(err).Errorf("couln't write input file %s. Aborting...", destinationFile)
						e.calcErrorChan <- calc.Name
						return
					}
				}
			}

			switch calc.Pipeline {
			case v1.VegaPipeline:
				vegaPipeline := pipelines.NewVegaPipeline(calc.Name, calcPath, calc.Spec.Params)

				controlFiles := filepath.Join(rootFolder, vegaPipeline.AtlasControlFiles)
				dataFiles := filepath.Join(rootFolder, vegaPipeline.AtlasDataFiles)

				// Creating symbolic links with the data/control files for atlas12_ada
				if err := e.createSymbolicLinks([]string{controlFiles, dataFiles}, calcPath); err != nil {
					e.logger.WithError(err).Error("coulnd't create symlinks for the vega pipeline")
					e.calcErrorChan <- calc.Name
					break
				}

				if err := vegaPipeline.Run(e.logger, e.stepUpdaterChan); err != nil {
					e.logger.WithError(err).Error("error while running the vega pipeline")
					e.calcErrorChan <- calc.Name
					break
				}

				data, err := os.ReadFile(filepath.Join(calcPath, "fort.7"))
				if err != nil {
					e.logger.WithError(err).Error("couldn't read the fort.7 file")
					e.calcErrorChan <- calc.Name
					break
				}

				params := map[string]string{
					"teff":  fmt.Sprintf("%f", calc.Spec.Params.Teff),
					"log_g": fmt.Sprintf("%f", calc.Spec.Params.LogG),
				}

				reply, err := e.grpcClient.StoreData(params, string(data))
				if err != nil {
					e.logger.WithError(err).Error("error while storing the data")
					e.calcErrorChan <- calc.Name
					break
				}

				e.logger.Infof("gRPC server response: %s", reply.GetMessage())
			default:
				for index, step := range calc.Spec.Steps {
					if len(step.Status) != 0 {
						continue
					}

					var status v1.CalculationPhase
					var cmdErr error
					status = v1.CompletedPhase

					ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
					defer cancel()

					cmd := exec.CommandContext(ctx, step.Command, step.Args...)
					cmd.Dir = calcPath

					fields := logrus.Fields{"command": cmd.Args, "step": index}
					e.logger.WithFields(fields).Info("Running command and waiting for it to finish...")

					combinedOut, err := cmd.CombinedOutput()
					if err != nil {
						e.logger.WithError(err).WithField("output", string(combinedOut)).Error("command failed...")
						status = v1.FailedPhase
						cmdErr = err
					}

					if err := e.dumpCommandOutput(calcPath, index, combinedOut); err != nil {
						e.logger.WithError(err).Error("couldn't dump command output to file")
					}

					result := util.Result{
						CalcName:     calc.Name,
						Step:         index,
						StdoutStderr: string(combinedOut),
						Status:       status,
						CommandError: cmdErr,
					}

					e.logger.WithFields(fields).WithField("status", status).Info("Command finished")
					e.stepUpdaterChan <- result

					if status == v1.FailedPhase {
						e.calcErrorChan <- calc.Name
					}
				}
				if err := copyMatchingFiles(calcPath, filepath.Join(rootFolder, calc.Labels[util.CalculationNameLabel]), calc.OutputFilesRegex); err != nil {
					e.logger.WithError(err).Error("couldn't copy the output files")
					e.calcErrorChan <- calc.Name
				}

			}

			// All steps finished. Update worker in workerpool and cleanup
			e.logger.WithField("calc-path", calcPath).Info("All steps finished. Cleaning up...")
			if err := os.RemoveAll(calcPath); err != nil {
				e.logger.WithField("path", calcPath).WithError(err).Error("couldn't remove the temp directory")
				e.calcErrorChan <- calc.Name
			}
			if err := util.UpdateWorkerStatusInPool(e.ctx, e.client, e.workerPool, e.nodename, e.namespace, workersv1.WorkerAvailableState); err != nil {
				// TODO: retry until the state is updated, otherwise the worker will deadlock
				panic(fmt.Errorf("failed to update worker's state in worker pool: %w", err))
			}
		}
	}
}

func (e *Executor) dumpCommandOutput(calcPath string, step int, data []byte) error {
	outFile := filepath.Join(calcPath, fmt.Sprintf("step-%d", step))
	e.logger.WithField("filename", outFile).WithField("path", calcPath).Info("Dumping command output to a file")
	if err := os.WriteFile(outFile, data, 0777); err != nil {
		return fmt.Errorf("couldn't generate the command output file: %v", err)
	}
	return nil
}

func (e *Executor) createSymbolicLinks(paths []string, toPath string) error {
	for _, path := range paths {
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				e.logger.WithError(err).Errorf("prevent panic by handling failure accessing a path %q", path)
				return err
			}
			if !info.IsDir() {
				if err := os.Symlink(path, filepath.Join(toPath, filepath.Base(path))); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			e.logger.WithField("path", path).WithError(err).Error("error while walking to path")
			return err
		}
	}
	return nil
}

func setUnlimitStack() error {
	var rLimit unix.Rlimit
	rLimit.Max = 18446744073709551615
	rLimit.Cur = 18446744073709551615

	if err := unix.Setrlimit(unix.RLIMIT_STACK, &rLimit); err != nil {
		return fmt.Errorf("error Setting Rlimit %v", err)
	}
	return nil
}

func copyFile(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}
	return nil
}

func copyMatchingFiles(srcDir, destDir, pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && regex.MatchString(info.Name()) {
			destPath := filepath.Join(destDir, info.Name())
			if err := copyFile(path, destPath); err != nil {
				return err
			}
			logrus.Infof("Copied %s to %s", path, destPath)
		}

		return nil
	})
}
