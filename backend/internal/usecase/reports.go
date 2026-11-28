package usecase

import (
	"context"
	"net/url"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type ReportStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListOwnedReports(context.Context, string) ([]domain.ReportSummary, error)
	ListOwnedReportsBefore(context.Context, string, domain.ReportCursor) ([]domain.ReportSummary, error)
	OwnedReport(context.Context, string, string) (domain.Report, error)
}

type Reports struct{ Store ReportStore }

func (uc Reports) List(ctx context.Context, digest [32]byte, rawQuery string) (domain.ReportPage, error) {
	empty := domain.ReportPage{}
	if ctx.Err() != nil || uc.Store == nil {
		return empty, domain.ErrReportStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return empty, err
	}
	// Query validation follows session authentication, even for malformed escapes.
	var cursor *domain.ReportCursor
	if rawQuery != "" {
		if len(rawQuery) > 512 {
			return empty, domain.ErrReportCursor
		}
		query, err := url.ParseQuery(rawQuery)
		if err != nil || len(query) != 1 || len(query["cursor"]) != 1 {
			return empty, domain.ErrReportCursor
		}
		cursor, err = domain.DecodeReportCursor(query.Get("cursor"))
		if err != nil {
			return empty, err
		}
	}
	var rows []domain.ReportSummary
	if cursor == nil {
		rows, err = uc.Store.ListOwnedReports(ctx, userID)
	} else {
		rows, err = uc.Store.ListOwnedReportsBefore(ctx, userID, *cursor)
	}
	if ctx.Err() != nil || err != nil {
		return empty, domain.ErrReportStore
	}
	page := domain.ReportPage{Reports: rows}
	if page.Reports == nil {
		page.Reports = []domain.ReportSummary{}
	}
	if len(rows) > domain.ReportPageSize {
		page.Reports = rows[:domain.ReportPageSize]
		next := domain.EncodeReportCursor(page.Reports[domain.ReportPageSize-1])
		page.NextCursor = &next
	}
	return page, nil
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
