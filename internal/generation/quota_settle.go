package generation

import (
	"context"
	"log"

	"github.com/grapestree/fgrapery/grapery-agent/internal/agentauth"
	"github.com/grapestree/fgrapery/grapery-agent/internal/domain"
)

func (s *Service) settleQuota(ctx context.Context, status domain.RunStatus, tokens int) {
	if s.client == nil {
		return
	}
	claims, ok := agentauth.ClaimsFromContext(ctx)
	if !ok || claims == nil || claims.QuotaReservationID == "" {
		return
	}
	resID := claims.QuotaReservationID
	var err error
	switch status {
	case domain.RunStatusSucceeded:
		err = s.client.ConfirmQuotaReservation(ctx, resID, tokens)
	default:
		err = s.client.ReleaseQuotaReservation(ctx, resID)
	}
	if err != nil {
		log.Printf("quota settlement failed reservation=%s status=%s: %v", resID, status, err)
	}
}
