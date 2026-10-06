// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/marcodellemarche/zlatan/internal/core"
)

// The credential sweeper must find whoever stopped moving, and must never
// find a migration the runner is working on, however old its last write.
func TestListIdle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, u := range []string{"drive-only", "copying", "waiting-google", "uploading", "fresh"} {
		if _, err := db.EnsureMigration(ctx, u, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.SetDriveState(ctx, "drive-only", core.DriveDone, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetDriveState(ctx, "copying", core.DriveCopying, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetPhotosState(ctx, "waiting-google", core.PhotosAwaitingTakeout, ""); err != nil {
		t.Fatal(err)
	}
	// Waiting for an upload (site or kiosk) is work in flight: the files land
	// over days without touching the row, so it must never be swept, or its
	// credentials would go out from under an upload in progress.
	if _, err := db.SetPhotosState(ctx, "uploading", core.PhotosAwaitingUpload, ""); err != nil {
		t.Fatal(err)
	}

	// Everything was written just now: nothing is idle an hour ago.
	if users, err := db.ListIdle(ctx, time.Now().Add(-time.Hour)); err != nil || len(users) != 0 {
		t.Fatalf("ListIdle(an hour ago) = %v, %v; want none", users, err)
	}

	users, err := db.ListIdle(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(users)
	if want := []string{"drive-only", "fresh"}; !slices.Equal(users, want) {
		t.Errorf("ListIdle = %v, want %v: work in flight is never idle", users, want)
	}
}
