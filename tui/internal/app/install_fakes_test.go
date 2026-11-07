package app

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

type fakeProbe struct {
	installed    map[string]bool
	installedErr error
	explicit     []string
	explicitErr  error
	commands     map[string]bool
	shell        string
	files        map[string][]byte
	root         bool
}

func (p *fakeProbe) DetectGPUs(context.Context) ([]domain.PCIDevice, error) { return nil, nil }

func (p *fakeProbe) InstalledPackages(context.Context) (map[string]bool, error) {
	return p.installed, p.installedErr
}

func (p *fakeProbe) ExplicitPackages(context.Context) ([]string, error) {
	return p.explicit, p.explicitErr
}

func (p *fakeProbe) HasCommand(name string) bool { return p.commands[name] }

func (p *fakeProbe) LoginShell(context.Context) (string, error) { return p.shell, nil }

func (p *fakeProbe) ReadSystemFile(path string) ([]byte, error) {
	if content, ok := p.files[path]; ok {
		return content, nil
	}
	return nil, fs.ErrNotExist
}

func (p *fakeProbe) IsRoot() bool { return p.root }

// scriptedRunner records every step; handle, when set, decides the result and may block or emit progress.
type scriptedRunner struct {
	mu     sync.Mutex
	steps  []domain.Step
	handle func(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult
}

func (r *scriptedRunner) Run(ctx context.Context, s domain.Step, progress func(domain.StepProgress)) domain.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	if r.handle != nil {
		return r.handle(ctx, s, progress)
	}
	return domain.StepResult{Status: domain.StepOK}
}

func (r *scriptedRunner) ran() []domain.Step {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.steps)
}

// commandLabels lists the labels of every command step the runner received, quiet ones included.
func (r *scriptedRunner) commandLabels() []string {
	var labels []string
	for _, c := range stepsOfType[domain.CommandStep](r.ran()) {
		labels = append(labels, c.Label)
	}
	return labels
}

func (r *scriptedRunner) quietCalls() int {
	n := 0
	for _, c := range stepsOfType[domain.CommandStep](r.ran()) {
		if c.Quiet {
			n++
		}
	}
	return n
}

func failedStep(message string) domain.StepResult {
	return domain.StepResult{Status: domain.StepFailed, Err: errors.New(message)}
}

func sorted(items ...string) []string {
	out := slices.Clone(items)
	slices.Sort(out)
	return out
}
