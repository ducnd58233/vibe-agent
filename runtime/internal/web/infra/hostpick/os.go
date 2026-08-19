package hostpick

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

type osPicker struct{}

// OS returns the host dialog picker for this GOOS.
func OS() domain.HostPicker {
	return osPicker{}
}

func (osPicker) Pick(ctx context.Context, kind domain.PickKind) (string, error) {
	return pick(ctx, kind)
}
