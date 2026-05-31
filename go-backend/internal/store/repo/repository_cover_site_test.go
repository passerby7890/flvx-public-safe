package repo

import (
	"path/filepath"
	"testing"
	"time"

	"go-backend/internal/store/model"
)

func TestCoverDomainProfilesAndTunnelBindings(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "cover-site.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	now := time.Now().UnixMilli()
	profiles := []model.CoverDomainProfile{
		{
			Name:            "snow-a",
			Enabled:         1,
			SiteLabel:       "site-a",
			Domains:         "*.a.example.com\na.example.com",
			CertProfile:     "snow-a",
			TemplateProfile: "static",
			CreatedTime:     now,
			UpdatedTime:     now,
		},
		{
			Name:            "snow-b",
			Enabled:         1,
			SiteLabel:       "site-b",
			Domains:         "*.b.example.com\nb.example.com",
			CertProfile:     "snow-b",
			TemplateProfile: "static",
			CreatedTime:     now,
			UpdatedTime:     now,
		},
	}
	for i := range profiles {
		if err := r.UpsertCoverDomainProfile(&profiles[i]); err != nil {
			t.Fatalf("upsert profile %d: %v", i, err)
		}
	}

	all, err := r.ListCoverDomainProfiles()
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(all))
	}

	if err := r.ReplaceTunnelCoverBindings(10, []int64{all[0].ID, all[1].ID, all[0].ID}); err != nil {
		t.Fatalf("replace tunnel bindings: %v", err)
	}
	if err := r.ReplaceTunnelCoverBindings(11, []int64{all[0].ID}); err != nil {
		t.Fatalf("replace tunnel bindings second tunnel: %v", err)
	}

	bound, err := r.ListTunnelCoverBindingsByTunnelID(10)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bound) != 2 {
		t.Fatalf("expected duplicate profile ids to be collapsed, got %d", len(bound))
	}

	enabled, err := r.ListEnabledCoverDomainProfilesByTunnelIDs([]int64{10, 11})
	if err != nil {
		t.Fatalf("list enabled profiles: %v", err)
	}
	if len(enabled) != 2 {
		t.Fatalf("expected profiles to be de-duplicated across tunnels, got %d", len(enabled))
	}

	if err := r.DeleteCoverDomainProfile(all[0].ID); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	remainingBindings, err := r.ListTunnelCoverBindings()
	if err != nil {
		t.Fatalf("list remaining bindings: %v", err)
	}
	for _, binding := range remainingBindings {
		if binding.ProfileID == all[0].ID {
			t.Fatalf("deleted profile still has binding: %+v", binding)
		}
	}
}
