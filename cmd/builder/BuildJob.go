package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/develar/app-builder/pkg/util"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/develar/errors"
	"github.com/develar/go-fs-util"
	"github.com/electronuserland/electron-build-service/internal"
	"github.com/json-iterator/go"
	"go.uber.org/zap"
)

type BuildJob struct {
	id string

	projectDir   string
	queueAddTime time.Time

	buildRequest    *BuildRequest
	rawBuildRequest *string

	handler *BuildHandler

	// channel for status messages
	messages chan string
	// channel for complete (with any result - success or failure or timeout)
	// error - only internal error, not from electron-builder
	complete chan BuildJobResult

	logger *zap.Logger

	context context.Context
}

type BuildJobResult struct {
	error     error
	rawResult string
	fileSizes []int64
}

const outDirName = "out"

func (t *BuildJob) String() string {
	return t.id
}

func (t *BuildJob) Run(ctx context.Context) {
	t.context = ctx

	jobStartTime := time.Now()
	waitTime := jobStartTime.Sub(t.queueAddTime)
	t.logger.Info("job started", zap.Duration("waitTime", waitTime))
	t.messages <- fmt.Sprintf("job started (queue time: %s)", waitTime.Round(time.Millisecond))

	err := t.doBuild(ctx, jobStartTime)
	if err != nil {
		t.complete <- BuildJobResult{error: errors.WithStack(err)}
	}
}

func (t *BuildJob) doBuild(buildContext context.Context, jobStartTime time.Time) error {
	defer func() {
		r := recover()
		if r != nil {
			t.complete <- BuildJobResult{error: errors.Errorf("recovered", r)}
		}
	}()

	// where electron-builder creates temp files
	projectTempDir := filepath.Join(t.handler.tempDir, t.id)
	// t.handler.tempDir should be already created,
	err := os.Mkdir(projectTempDir, 0700)
	if err != nil {
		return errors.WithStack(err)
	}

	projectOutDir := filepath.Join(t.projectDir, outDirName)
	err = fsutil.EnsureEmptyDir(projectOutDir)
	if err != nil {
		return errors.WithStack(err)
	}

	if buildContext.Err() != nil {
		return buildContext.Err()
	}

	command := exec.CommandContext(buildContext, "node", t.handler.scriptPath, *t.rawBuildRequest)
	command.Env = append(os.Environ(),
		"PROJECT_DIR="+t.projectDir,
		"PROJECT_OUT_DIR="+projectOutDir,
		"APP_BUILDER_TMP_DIR="+projectTempDir,
		// we do cleanup in any case, no need to waste nodejs worker time
		"TMP_DIR_MANAGER_ENSURE_REMOVED_ON_EXIT=false",
		"FORCE_COLOR=0",
		"SNAP_DESTRUCTIVE_MODE=true",
	)
	command.Dir = t.projectDir

	err = t.doExecute(command)
	if err != nil {
		return err
	}

	if buildContext.Err() != nil {
		return buildContext.Err()
	}

	// reliable way to get result (since we cannot use out/err output)
	rawResult, err := ioutil.ReadFile(filepath.Join(projectTempDir, "__build-result.json"))
	if err != nil {
		return errors.WithStack(err)
	}

	info, err := ioutil.ReadFile(filepath.Join(t.projectDir, "info.json"))
	if err != nil {
		t.logger.Error("cannot write project info", zap.Error(err))
	}

	result, err := t.processResult(rawResult, projectOutDir)
	if err != nil {
		return err
	}

	t.logger.Info("job completed",
		zap.Duration("duration", time.Since(jobStartTime)),
		zap.ByteString("result", rawResult),
		zap.Int64s("fileSizes", result.fileSizes),
		zap.ByteString("projectInfo", info),
	)

	t.complete <- *result

	go t.removeAllFilesExceptArtifacts(projectTempDir)

	return nil
}

func (t *BuildJob) doExecute(command *exec.Cmd) error {
	r, w := io.Pipe()
	defer util.Close(r)
	defer util.Close(w)

	command.Stdout = w
	command.Stderr = w

	err := command.Start()
	if err != nil {
		return errors.WithStack(err)
	}

	go func() {
		outReader := bufio.NewReader(r)
		var b bytes.Buffer
		for {
			line, err := outReader.ReadString('\n')
			if err != nil {
				if err != io.EOF && err != io.ErrClosedPipe {
					t.logger.Error("cannot read builder output", zap.Error(err))
				}
				break
			}

			// do not send status if some new lines are already available
			if outReader.Buffered() > 0 {
				b.WriteString(line)
				continue
			} else if b.Len() == 0 {
				t.messages <- line
			} else {
				b.WriteString(line)
				t.messages <- b.String()
				b.Reset()
			}
		}

		// read rest (if no new line in the end)
		_, err = outReader.WriteTo(&b)
		if err != nil && err != io.EOF && err != io.ErrClosedPipe {
			t.logger.Error("cannot read builder output", zap.Error(err))
		}

		if b.Len() > 0 {
			t.messages <- b.String()
		}
	}()

	err = command.Wait()

	util.Close(r)
	util.Close(w)

	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (t *BuildJob) processResult(rawResult []byte, projectOutDir string) (*BuildJobResult, error) {
	result := &BuildJobResult{
		rawResult: string(rawResult),
	}

	if len(rawResult) > 0 && rawResult[0] == '[' {
		var partialArtifactInfo []PartialArtifactInfo
		err := jsoniter.Unmarshal(rawResult, &partialArtifactInfo)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		result.fileSizes, err = t.computeFileSizes(partialArtifactInfo, projectOutDir)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return result, nil
}

func (t *BuildJob) computeFileSizes(partialArtifactInfo []PartialArtifactInfo, projectOutDir string) ([]int64, error) {
	fileSizes := make([]int64, len(partialArtifactInfo))
	err := internal.MapAsync(len(partialArtifactInfo), t.logger, func(taskIndex int) (func() error, error) {
		file := filepath.Join(projectOutDir, partialArtifactInfo[taskIndex].File)
		return func() error {
			info, err := os.Stat(file)
			if err != nil {
				return err
			}

			fileSizes[taskIndex] = info.Size()
			return nil
		}, nil
	})
	return fileSizes, err
}

type PartialArtifactInfo struct {
	File string `json:"file"`
}

func (t *BuildJob) removeAllFilesExceptArtifacts(projectTempDir string) {
	removeFileAndLog(t.logger, projectTempDir)

	// on complete all project dir will be removed, but temp files are not required since now, so cleanup early because files can be on a RAM disk
	// yes, for now we expect the only target
	files, err := fsutil.ReadDirContent(t.projectDir)
	if err != nil {
		t.logger.Error("cannot remove", zap.Error(err))
	}

	for _, file := range files {
		if file == outDirName {
			continue
		}

		removeFileAndLog(t.logger, file)
	}
}

func removeFileAndLog(logger *zap.Logger, file string) {
	logger.Debug("remove file", zap.String("file", file))
	err := os.RemoveAll(file)
	if err != nil {
		logger.Error("cannot remove", zap.Error(err))
	}
}
