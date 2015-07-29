package backend

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/travis-ci/worker/config"
	gocontext "golang.org/x/net/context"
)

var (
	errNoScriptUploaded = fmt.Errorf("no script uploaded")
	localHelp           = map[string]string{
		"SCRIPTS_DIR": "directory where generated scripts will be written",
	}
)

func init() {
	config.SetProviderHelp("Local", localHelp)
}

type localProvider struct {
	cfg *config.ProviderConfig
}

func newLocalProvider(cfg *config.ProviderConfig) *localProvider {
	return &localProvider{cfg: cfg}
}

func (p *localProvider) Start(ctx gocontext.Context, startAttributes *StartAttributes) (Instance, error) {
	return &localInstance{p: p}, nil
}

type localInstance struct {
	p *localProvider

	scriptsDir string
	scriptPath string
}

func newLocalInstance(p *localProvider) (*localInstance, error) {
	scriptsDir, _ := os.Getwd()

	if p.cfg.IsSet("SCRIPTS_DIR") {
		scriptsDir = p.cfg.Get("SCRIPTS_DIR")
	}

	if scriptsDir == "" {
		scriptsDir = os.TempDir()
	}

	return &localInstance{
		p:          p,
		scriptsDir: scriptsDir,
	}, nil
}

func (i *localInstance) UploadScript(ctx gocontext.Context, script []byte) error {
	scriptPath := filepath.Join(i.scriptsDir, fmt.Sprintf("build-%v.sh", time.Now().UTC().UnixNano()))
	f, err := os.Create(scriptPath)
	if err != nil {
		return err
	}

	i.scriptPath = scriptPath

	scriptBuf := bytes.NewBuffer(script)
	_, err = io.Copy(f, scriptBuf)
	return err
}

func (i *localInstance) RunScript(ctx gocontext.Context, writer io.Writer) (*RunResult, error) {
	if i.scriptPath == "" {
		return &RunResult{Completed: false}, errNoScriptUploaded
	}

	cmd := exec.Command(fmt.Sprintf("bash %s", i.scriptPath))
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Start()
	if err != nil {
		return &RunResult{Completed: false}, err
	}

	errChan := make(chan error)
	go func() {
		errChan <- cmd.Wait()
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return &RunResult{Completed: false}, err
		}
		return &RunResult{Completed: true}, nil
	case <-ctx.Done():
		err = ctx.Err()
		if err != nil {
			return &RunResult{Completed: false}, err
		}
		return &RunResult{Completed: true}, nil
	}
	panic("no cases matched???")
}

func (i *localInstance) Stop(ctx gocontext.Context) error {
	return nil
}

func (i *localInstance) ID() string {
	return fmt.Sprintf("local:%s", i.scriptPath)
}
