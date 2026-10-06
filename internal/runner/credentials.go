// SPDX-License-Identifier: AGPL-3.0-or-later

package runner

import (
	"context"
	"time"

	"github.com/marcodellemarche/zlatan/internal/core"
	"github.com/marcodellemarche/zlatan/internal/immich"
	"github.com/marcodellemarche/zlatan/internal/nextcloud"
)

// googleProvider is the key the Google token is stored under.
const googleProvider = "google"

// A credential is kept only while a track can still use it. Each one belongs
// to a track, except the Google token, which both use (the Drive copy, and the
// "Add to Drive" route of Photos):
//
//   - Drive done: the Nextcloud app password is revoked and forgotten.
//   - Photos done: the Immich key is forgotten. Immich offers no way to revoke
//     a key without its id, so the person deletes it in Immich; the
//     notification tells them.
//   - Both done: the Google token is revoked at Google and forgotten.
//   - Nothing moved for the retention period and no work in flight: everything
//     is forgotten. That covers whoever moves only one half, and whoever stops
//     halfway, who would otherwise leave a token here for good.
//
// A failed track keeps its credentials, so "try again" works without
// reconnecting; the idle sweep is what eventually clears an abandoned one.

// releaseFinished forgets the credentials no track needs any more, after a
// track reached done. Failures are logged, never returned: the migration
// already succeeded, and a credential left behind is swept later.
func (r *Runner) releaseFinished(ctx context.Context, user string) {
	m, err := r.store.GetMigration(ctx, user)
	if err != nil {
		r.log.Error("release credentials: read migration", "user", user, "error", err)
		return
	}
	driveDone := m.DriveState == core.DriveDone
	photosDone := m.PhotosState == core.PhotosDone
	if driveDone {
		r.forgetNextcloud(ctx, user)
	}
	if photosDone {
		r.forget(ctx, user, immich.Provider)
	}
	if driveDone && photosDone {
		r.forgetGoogle(ctx, user)
	}
}

// forgetAll forgets every credential a person has given.
func (r *Runner) forgetAll(ctx context.Context, user string) {
	r.forgetNextcloud(ctx, user)
	r.forget(ctx, user, immich.Provider)
	r.forgetGoogle(ctx, user)
}

// forgetGoogle revokes the Google grant and forgets the token. Revoking is
// best effort: forgetting it is what takes away Zlatan's access, and revoking
// also removes the grant from the person's Google account. A token that no
// longer opens is still forgotten.
func (r *Runner) forgetGoogle(ctx context.Context, user string) {
	tok, err := r.store.GetToken(ctx, user, googleProvider)
	if err != nil {
		return
	}
	if r.google != nil {
		if tokens, err := r.openTokens(tok); err == nil && tokens.RefreshToken != "" {
			rctx, cancel := context.WithTimeout(ctx, probeTimeout)
			if err := r.google.Revoke(rctx, tokens.RefreshToken); err != nil {
				r.log.Warn("release credentials: revoke the Google grant", "user", user, "error", err)
			}
			cancel()
		}
	}
	r.forget(ctx, user, googleProvider)
}

// forgetNextcloud deletes the app password in Nextcloud, so it stops working
// there too, and forgets it. The Login Flow token that may be left from an
// unfinished grant goes with it.
func (r *Runner) forgetNextcloud(ctx context.Context, user string) {
	if tok, err := r.store.GetToken(ctx, user, nextcloud.Provider); err == nil {
		if r.nc != nil {
			if creds, err := nextcloud.OpenCredentials(r.sealer, tok.Sealed); err == nil {
				rctx, cancel := context.WithTimeout(ctx, probeTimeout)
				if err := r.nc.RevokeAppPassword(rctx, creds); err != nil {
					r.log.Warn("release credentials: delete the Nextcloud app password", "user", user, "error", err)
				}
				cancel()
			}
		}
		r.forget(ctx, user, nextcloud.Provider)
	}
	r.forget(ctx, user, nextcloudFlowProvider)
}

func (r *Runner) forget(ctx context.Context, user, provider string) {
	if _, err := r.store.GetToken(ctx, user, provider); err != nil {
		return
	}
	if err := r.store.DeleteToken(ctx, user, provider); err != nil {
		r.log.Error("release credentials: delete", "user", user, "provider", provider, "error", err)
		return
	}
	r.log.Info("credential forgotten", "user", user, "provider", provider)
}

// sweepIdle forgets the credentials of everyone whose migration has not moved
// for longer than idle and has no work in flight.
func (r *Runner) sweepIdle(ctx context.Context, idle time.Duration) {
	users, err := r.store.ListIdle(ctx, time.Now().Add(-idle))
	if err != nil {
		r.log.Error("credential sweeper: list", "error", err)
		return
	}
	for _, user := range users {
		r.forgetAll(ctx, user)
	}
}
