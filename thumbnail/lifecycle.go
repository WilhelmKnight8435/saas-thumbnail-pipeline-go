package thumbnail

import "fmt"

type AccountState string

const (
	AccountOnboarding AccountState = "onboarding"
	AccountActive     AccountState = "active"
	AccountSuspended  AccountState = "suspended"
)

type Tenant struct {
	ID               string       `json:"id"`
	State            AccountState `json:"state"`
	OnboardingDone   bool         `json:"onboarding_done"`
	ThumbnailProfile string       `json:"thumbnail_profile"`
}

type Variant struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Fit    string `json:"fit"`
	Format string `json:"format"`
}

func VariantsFor(t Tenant) ([]Variant, error) {
	if !t.OnboardingDone {
		return nil, fmt.Errorf("tenant %s has not completed onboarding", t.ID)
	}
	if t.State != AccountActive {
		return nil, fmt.Errorf("tenant %s account is %s", t.ID, t.State)
	}

	switch t.ThumbnailProfile {
	case "catalog":
		return []Variant{
			{Name: "card", Width: 640, Height: 360, Fit: "cover", Format: "webp"},
			{Name: "list", Width: 240, Height: 240, Fit: "cover", Format: "webp"},
		}, nil
	case "avatar":
		return []Variant{{Name: "avatar", Width: 256, Height: 256, Fit: "cover", Format: "webp"}}, nil
	default:
		return nil, fmt.Errorf("tenant %s has unknown thumbnail profile %q", t.ID, t.ThumbnailProfile)
	}
}
