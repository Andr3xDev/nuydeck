package backup

import (
	"context"
	"errors"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestCheckRestorable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Backup)
		want   error
	}{
		{"complete backup is accepted", func(*Backup) {}, nil},
		{"incomplete backup is refused", func(b *Backup) { b.Incomplete = true }, ErrIncomplete},
		{"unreadable backup is refused", func(b *Backup) { b.Unreadable = true }, ErrUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testBackup()
			tt.mutate(&b)
			if err := CheckRestorable(b); !errors.Is(err, tt.want) {
				t.Errorf("CheckRestorable = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPlanRestore_RefusesBackupsThatAreNotRestorableBeforeDiffing(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Backup)
		want   error
	}{
		{"incomplete", func(b *Backup) { b.Incomplete = true }, ErrIncomplete},
		{"unreadable", func(b *Backup) { b.Unreadable = true }, ErrUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testBackup()
			tt.mutate(&b)
			in := &fakeInspector{diff: diffReturning(change("hypr/a.conf", plan.Update, plan.KindFile))}

			_, err := newService(t, in, &fakeRunner{}).PlanRestore(context.Background(), b)

			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if len(in.diffCalls) != 0 {
				t.Errorf("diff ran %d times for a refused backup", len(in.diffCalls))
			}
		})
	}
}
