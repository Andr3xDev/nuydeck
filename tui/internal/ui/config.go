package ui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	configUpdateLabel = "Update TUI"
	restartNoteText   = "Restart nuydeck to run the new version."
)

// Config shows the repo path, the backups directory and the dry-run state, and offers the TUI self-update.
type Config struct {
	svc  *shell
	flow *flow
}

func newConfig(svc *shell) *Config { return &Config{svc: svc} }

func (c *Config) Title() string  { return "Config" }
func (c *Config) Enter() tea.Cmd { return nil }

func (c *Config) canUpdate() bool { return c.svc.Config.PlanUpdate != nil && c.svc.RunPlan != nil }

func (c *Config) Keys() []key.Binding {
	switch {
	case c.flow.active():
		return c.flow.Keys()
	case c.canUpdate():
		return []key.Binding{described(c.svc.keys.Confirm, "update TUI")}
	}
	return nil
}

func (c *Config) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case plannedMsg:
		if c.flow.active() {
			return c, c.flow.Update(msg)
		}
	case tea.KeyPressMsg:
		if c.flow.active() {
			return c, c.flow.Update(msg)
		}
		if c.canUpdate() && key.Matches(msg, c.svc.keys.Confirm) {
			return c, c.update()
		}
	}
	return c, nil
}

func (c *Config) update() tea.Cmd {
	planUpdate := c.svc.Config.PlanUpdate
	c.flow = newFlow(c.svc, flowSource{
		Title: configUpdateLabel,
		Plan:  func(context.Context, []string) (staged, error) { return newPlanStaged(planUpdate()), nil },
		Run: func(p plan.Plan) runRequest {
			req := planRunRequest(c.svc, p)
			run := req.work
			req.work = func(ctx context.Context, emit func(plan.Event)) runEnd {
				end := run(ctx, emit)
				// A dry-run executes nothing, so there is no new version to restart into.
				if end.err == nil && !c.svc.dryRun {
					emit(plan.Event{Kind: plan.EventLine, Line: restartNoteText})
				}
				return end
			}
			return req
		},
	})
	return c.flow.start()
}

func (c *Config) View(width, height int) string {
	if c.flow.active() {
		return c.flow.box(c.Title(), width, height)
	}
	th, cw := c.svc.th, contentWidth(width)
	dryRun := envOffLabel
	if c.svc.dryRun {
		dryRun = envOnLabel
	}
	lines := []string{
		truncate(sanitizeLine(fmt.Sprintf("Repo path:   %s", c.svc.Config.RepoPath)), cw),
		truncate(sanitizeLine(fmt.Sprintf("Backups dir: %s", c.svc.Config.BackupsDir)), cw),
		fmt.Sprintf("Dry-run:     %s", dryRun),
	}
	if c.canUpdate() {
		lines = append(lines, "", th.title.Render(configUpdateLabel), th.muted.Render("enter updates nuydeck"))
	}
	return screenBox(th, c.Title(), lines, width, height)
}
