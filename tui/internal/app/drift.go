package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

type DriftState int

const (
	DriftInSync DriftState = iota
	DriftDrifted
	DriftError
)

// Missing says which side of a drifted entry does not exist.
type Missing int

const (
	MissingNone Missing = iota
	MissingInRepo
	MissingOnPC
)

// DriftConcurrency bounds simultaneous entry diffs.
const DriftConcurrency = 4

type DriftResult struct {
	Entry      manifest.ResolvedEntry
	Missing    Missing
	State      DriftState
	Changes    int
	Err        error
	RepoTimes  plan.RepoTimes
	PCTime     time.Time
	LastBackup *backup.Backup
}

// Drift diffs repo to PC per entry and streams one result per entry; the channel closes when all are done.
func (s *Service) Drift(ctx context.Context, entries []manifest.ResolvedEntry) <-chan DriftResult {
	out := make(chan DriftResult, len(entries))
	var lastBackup *backup.Backup
	backupDone := make(chan struct{})
	go func() {
		defer close(backupDone)
		lastBackup = s.lastCompleteBackup(ctx)
	}()
	slots := make(chan struct{}, DriftConcurrency)
	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
			}
			if err := ctx.Err(); err != nil {
				out <- DriftResult{Entry: e, State: DriftError, Err: err}
				return
			}
			r := s.driftEntry(ctx, e)
			<-backupDone
			r.LastBackup = lastBackup
			out <- r
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (s *Service) driftEntry(ctx context.Context, e manifest.ResolvedEntry) DriftResult {
	r := DriftResult{Entry: e}
	repoKind, err := s.Inspector.PathKind(e.RepoPath)
	if err != nil {
		r.State, r.Err = DriftError, err
		return r
	}
	pcKind, err := s.Inspector.PathKind(e.DestPath)
	if err != nil {
		r.State, r.Err = DriftError, err
		return r
	}
	switch {
	case repoKind == plan.KindNone && pcKind == plan.KindNone:
		r.State, r.Err = DriftError, errors.New("absent on both sides")
		return r
	case repoKind == plan.KindNone:
		r.State, r.Missing = DriftDrifted, MissingInRepo
		return r
	case pcKind == plan.KindNone:
		r.State, r.Missing = DriftDrifted, MissingOnPC
		return r
	}
	changes, err := s.Inspector.Diff(ctx, e, plan.Endpoint{Path: e.RepoPath}, plan.Endpoint{Path: e.DestPath})
	if err != nil {
		r.State, r.Err = DriftError, err
		return r
	}
	if r.Changes = len(changes); r.Changes > 0 {
		r.State = DriftDrifted
	}
	r.RepoTimes, _ = s.Inspector.RepoTimes(ctx, s.RepoDir, string(e.Root)+"/"+e.Path)
	r.PCTime, _ = s.Inspector.NewestMTime(ctx, e.DestPath, e.Filter)
	return r
}

func (s *Service) lastCompleteBackup(ctx context.Context) *backup.Backup {
	backups, err := s.Inspector.ListBackups(ctx)
	if err != nil {
		return nil
	}
	for _, b := range backups {
		if !b.Incomplete && !b.Unreadable {
			return &b
		}
	}
	return nil
}
