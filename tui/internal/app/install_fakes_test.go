package app

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type fakeProbe struct {
	installed    map[string]bool
	installedErr error
	commands     map[string]bool
	shell        string
	files        map[string][]byte
	root         bool
}

func (p *fakeProbe) DetectGPUs(context.Context) ([]domain.PCIDevice, error) { return nil, nil }

func (p *fakeProbe) InstalledPackages(context.Context) (map[string]bool, error) {
	return p.installed, p.installedErr
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
	steps  []plan.Step
	handle func(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult
}

func (r *scriptedRunner) Run(ctx context.Context, s plan.Step, progress func(plan.StepProgress)) plan.StepResult {
	r.mu.Lock()
	r.steps = append(r.steps, s)
	r.mu.Unlock()
	if r.handle != nil {
		return r.handle(ctx, s, progress)
	}
	return plan.StepResult{Status: plan.StepOK}
}

func (r *scriptedRunner) ran() []plan.Step {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.steps)
}

// commandLabels lists the labels of every command step the runner received, quiet ones included.
func (r *scriptedRunner) commandLabels() []string {
	var labels []string
	for _, c := range stepsOfType[plan.CommandStep](r.ran()) {
		labels = append(labels, c.Label)
	}
	return labels
}

func (r *scriptedRunner) quietCalls() int {
	n := 0
	for _, c := range stepsOfType[plan.CommandStep](r.ran()) {
		if c.Quiet {
			n++
		}
	}
	return n
}

func failedStep(message string) plan.StepResult {
	return plan.StepResult{Status: plan.StepFailed, Err: errors.New(message)}
}

func sorted(items ...string) []string {
	out := slices.Clone(items)
	slices.Sort(out)
	return out
}
