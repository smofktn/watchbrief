package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type ReportStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListOwnedReports(context.Context, string) ([]domain.ReportSummary, error)
	OwnedReport(context.Context, string, string) (domain.Report, error)
}

type Reports struct{ Store ReportStore }

func (uc Reports) List(ctx context.Context, digest [32]byte) ([]domain.ReportSummary, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return nil, domain.ErrReportStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return nil, err
	}
	rows, err := uc.Store.ListOwnedReports(ctx, userID)
	if ctx.Err() != nil {
		return nil, domain.ErrReportStore
	}
	return rows, err
}

func (uc Reports) Get(ctx context.Context, digest [32]byte, id string) (domain.Report, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.Report{}, domain.ErrReportStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return domain.Report{}, err
	}
	if !domain.ValidIdentifier(id) {
		return domain.Report{}, domain.ErrInvalidIdentifier
	}
	value, err := uc.Store.OwnedReport(ctx, userID, id)
	if ctx.Err() != nil {
		return domain.Report{}, domain.ErrReportStore
	}
	return value, err
}
