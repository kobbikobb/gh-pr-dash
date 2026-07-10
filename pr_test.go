package main

import (
	"testing"
	"time"
)

func prWith(f func(*ghPR)) ghPR {
	pr := ghPR{Number: 1, Mergeable: "MERGEABLE"}
	pr.Repository.NameWithOwner = "owner/repo"
	f(&pr)
	return pr
}

func setRollup(pr *ghPR, state string) {
	pr.Commits.Nodes = []struct {
		Commit struct {
			StatusCheckRollup *struct{ State string }
		}
	}{{}}
	pr.Commits.Nodes[0].Commit.StatusCheckRollup = &struct{ State string }{State: state}
}

func TestCiCode(t *testing.T) {
	cases := map[string]string{
		"SUCCESS": "ok", "FAILURE": "fail", "ERROR": "fail",
		"PENDING": "pending", "EXPECTED": "pending", "": "none", "WEIRD": "none",
	}
	for in, want := range cases {
		t.Run("should map "+in+" to "+want, func(t *testing.T) {
			if got := ciCode(in); got != want {
				t.Errorf("ciCode(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestMergeCode(t *testing.T) {
	cases := map[string]string{
		"CONFLICTING": "conflict",
		"MERGEABLE":   "clean",
		"UNKNOWN":     "unknown",
		"":            "unknown",
	}
	for in, want := range cases {
		t.Run("should map "+in+" to "+want, func(t *testing.T) {
			if got := mergeCode(in); got != want {
				t.Errorf("mergeCode(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestRollupState(t *testing.T) {
	t.Run("should return state from rollup", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { setRollup(p, "SUCCESS") })

		if got := rollupState(pr); got != "SUCCESS" {
			t.Errorf("got %q, want SUCCESS", got)
		}
	})

	t.Run("should return empty when no commits", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.Commits.Nodes = nil })

		if got := rollupState(pr); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("should return empty when rollup is nil", func(t *testing.T) {
		pr := prWith(func(p *ghPR) {
			p.Commits.Nodes = []struct {
				Commit struct {
					StatusCheckRollup *struct{ State string }
				}
			}{{}}
		})

		if got := rollupState(pr); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestReviewCode(t *testing.T) {
	t.Run("should return none for drafts even when approved", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.IsDraft = true; p.ReviewDecision = "APPROVED" })

		if got := reviewCode(pr); got != "none" {
			t.Errorf("got %q, want none", got)
		}
	})

	t.Run("should honor a standing approval when reviewDecision is empty", func(t *testing.T) {
		pr := prWith(func(p *ghPR) {
			p.LatestOpinionatedReviews.Nodes = []struct{ State string }{{State: "APPROVED"}}
		})

		if got := reviewCode(pr); got != "approved" {
			t.Errorf("got %q, want approved", got)
		}
	})

	t.Run("should prefer changes over approved", func(t *testing.T) {
		pr := prWith(func(p *ghPR) {
			p.ReviewDecision = "APPROVED"
			p.LatestOpinionatedReviews.Nodes = []struct{ State string }{{State: "CHANGES_REQUESTED"}}
		})

		if got := reviewCode(pr); got != "changes" {
			t.Errorf("got %q, want changes", got)
		}
	})
}

func TestTierFor(t *testing.T) {
	t.Run("should rank CI failure as needs-action", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { setRollup(p, "FAILURE") })

		row := classify(pr, time.Now())

		if row.Tier != tierNeedsAction {
			t.Errorf("tier = %d, want %d", row.Tier, tierNeedsAction)
		}
	})

	t.Run("should rank conflict as needs-action even when CI is green", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.Mergeable = "CONFLICTING"; setRollup(p, "SUCCESS") })

		if classify(pr, time.Now()).Tier != tierNeedsAction {
			t.Error("conflict should be needs-action")
		}
	})

	t.Run("should rank approved+green+clean as ready", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.ReviewDecision = "APPROVED"; setRollup(p, "SUCCESS") })

		if classify(pr, time.Now()).Tier != tierReady {
			t.Error("want ready")
		}
	})

	t.Run("should rank approved+clean with no checks and a clean merge state as ready", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.ReviewDecision = "APPROVED"; p.MergeStateStatus = "CLEAN" })

		if classify(pr, time.Now()).Tier != tierReady {
			t.Error("want ready when there are no checks to wait on")
		}
	})

	t.Run("should rank approved+clean blocked by unreported checks as waiting-on-CI", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.ReviewDecision = "APPROVED"; p.MergeStateStatus = "BLOCKED" })

		if classify(pr, time.Now()).Tier != tierBuilding {
			t.Error("want waiting-on-CI when required checks have not reported")
		}
	})

	t.Run("should rank approved+clean with pending CI as waiting-on-CI", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.ReviewDecision = "APPROVED"; setRollup(p, "PENDING") })

		if classify(pr, time.Now()).Tier != tierBuilding {
			t.Error("want waiting-on-CI")
		}
	})

	t.Run("should rank review-required as waiting", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.ReviewDecision = "REVIEW_REQUIRED"; setRollup(p, "SUCCESS") })

		if classify(pr, time.Now()).Tier != tierWaiting {
			t.Error("want waiting")
		}
	})

	t.Run("should rank drafts last", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.IsDraft = true })

		if classify(pr, time.Now()).Tier != tierDrafts {
			t.Error("want drafts")
		}
	})
}

func TestClassify(t *testing.T) {
	t.Run("should compute idle days and short ref", func(t *testing.T) {
		now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
		pr := prWith(func(p *ghPR) {
			p.Number = 42
			p.UpdatedAt = now.Add(-72 * time.Hour)
		})

		row := classify(pr, now)

		if row.IdleDays != 3 {
			t.Errorf("idle = %d, want 3", row.IdleDays)
		}
		if row.Ref != "repo#42" {
			t.Errorf("ref = %q, want repo#42", row.Ref)
		}
	})
}

func TestBuildMergedRows(t *testing.T) {
	now := time.Now()

	t.Run("should mark merged PRs with the merged tier and marker", func(t *testing.T) {
		pr := prWith(func(p *ghPR) { p.Number = 7; p.MergedAt = now.Add(-2 * time.Hour) })

		rows := buildMergedRows([]ghPR{pr}, "", now)

		if len(rows) != 1 {
			t.Fatalf("got %d rows", len(rows))
		}
		if rows[0].Tier != tierMerged || rows[0].Merge != "merged" {
			t.Errorf("tier = %d, merge = %q", rows[0].Tier, rows[0].Merge)
		}
	})

	t.Run("should keep search order (newest first)", func(t *testing.T) {
		newer := prWith(func(p *ghPR) { p.Number = 1 })
		older := prWith(func(p *ghPR) { p.Number = 2 })

		rows := buildMergedRows([]ghPR{newer, older}, "", now)

		if rows[0].Ref != "repo#1" || rows[1].Ref != "repo#2" {
			t.Errorf("order changed: %q, %q", rows[0].Ref, rows[1].Ref)
		}
	})

	t.Run("should filter by owner prefix", func(t *testing.T) {
		mine := prWith(func(p *ghPR) { p.Repository.NameWithOwner = "acme/api" })
		other := prWith(func(p *ghPR) { p.Repository.NameWithOwner = "other/api" })

		rows := buildMergedRows([]ghPR{mine, other}, "acme", now)

		if len(rows) != 1 || rows[0].Ref != "api#1" {
			t.Errorf("owner filter failed: %+v", rows)
		}
	})
}

func TestBuildRows(t *testing.T) {
	now := time.Now()

	t.Run("should sort by tier then oldest-first within tier", func(t *testing.T) {
		ready := prWith(func(p *ghPR) { p.Number = 1; p.ReviewDecision = "APPROVED"; setRollup(p, "SUCCESS") })
		failNew := prWith(func(p *ghPR) { p.Number = 2; p.UpdatedAt = now.Add(-24 * time.Hour); setRollup(p, "FAILURE") })
		failOld := prWith(func(p *ghPR) { p.Number = 3; p.UpdatedAt = now.Add(-240 * time.Hour); setRollup(p, "FAILURE") })

		rows := buildRows([]ghPR{ready, failNew, failOld}, "", now)

		if len(rows) != 3 {
			t.Fatalf("got %d rows", len(rows))
		}
		if rows[0].Ref != "repo#3" || rows[1].Ref != "repo#2" {
			t.Errorf("needs-action not oldest-first: %q, %q", rows[0].Ref, rows[1].Ref)
		}
		if rows[2].Tier != tierReady {
			t.Errorf("ready should sort last, got tier %d", rows[2].Tier)
		}
	})

	t.Run("should filter by owner prefix", func(t *testing.T) {
		mine := prWith(func(p *ghPR) { p.Repository.NameWithOwner = "acme/api" })
		other := prWith(func(p *ghPR) { p.Repository.NameWithOwner = "other/api" })

		rows := buildRows([]ghPR{mine, other}, "acme", now)

		if len(rows) != 1 || rows[0].Ref != "api#1" {
			t.Errorf("owner filter failed: %+v", rows)
		}
	})

	t.Run("should skip non-PR nodes", func(t *testing.T) {
		rows := buildRows([]ghPR{{Number: 0}}, "", now)

		if len(rows) != 0 {
			t.Errorf("expected empty, got %d", len(rows))
		}
	})
}
