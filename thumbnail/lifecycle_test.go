package thumbnail

import "testing"

func TestVariantsForAccountLifecycle(t *testing.T) {
	tests := []struct {
		name      string
		tenant    Tenant
		wantNames []string
		wantError bool
	}{
		{name: "active catalog tenant gets two responsive variants", tenant: Tenant{ID: "acme", State: AccountActive, OnboardingDone: true, ThumbnailProfile: "catalog"}, wantNames: []string{"card", "list"}},
		{name: "active avatar tenant gets square variant", tenant: Tenant{ID: "acme", State: AccountActive, OnboardingDone: true, ThumbnailProfile: "avatar"}, wantNames: []string{"avatar"}},
		{name: "onboarding blocks processing", tenant: Tenant{ID: "acme", State: AccountOnboarding, ThumbnailProfile: "catalog"}, wantError: true},
		{name: "admin suspension blocks processing", tenant: Tenant{ID: "acme", State: AccountSuspended, OnboardingDone: true, ThumbnailProfile: "catalog"}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VariantsFor(tt.tenant)
			if (err != nil) != tt.wantError {
				t.Fatalf("VariantsFor() error = %v, wantError %v", err, tt.wantError)
			}
			if len(got) != len(tt.wantNames) {
				t.Fatalf("got %d variants, want %d", len(got), len(tt.wantNames))
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("variant %d name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}
