package usecase

import (
	"context"
	"net/url"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type GenerationJobStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListOwnedGenerationJobs(context.Context, string) ([]domain.GenerationJobSummary, error)
	ListOwnedGenerationJobsBefore(context.Context, string, domain.GenerationJobCursor) ([]domain.GenerationJobSummary, error)
}

type GenerationJobs struct{ Store GenerationJobStore }

func (uc GenerationJobs) List(ctx context.Context, digest [32]byte, rawQuery string) (domain.GenerationJobPage, error) {
	empty := domain.GenerationJobPage{}
	if ctx.Err() != nil || uc.Store == nil {
		return empty, domain.ErrGenerationJobStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return empty, err
	}
	// Query validation follows session authentication, even for malformed escapes.
	var cursor *domain.GenerationJobCursor
	if rawQuery != "" {
		if len(rawQuery) > 512 {
			return empty, domain.ErrGenerationJobCursor
		}
		query, err := url.ParseQuery(rawQuery)
		if err != nil || len(query) != 1 || len(query["cursor"]) != 1 {
			return empty, domain.ErrGenerationJobCursor
		}
		cursor, err = domain.DecodeGenerationJobCursor(query.Get("cursor"))
		if err != nil {
			return empty, err
		}
	}
	var rows []domain.GenerationJobSummary
	if cursor == nil {
		rows, err = uc.Store.ListOwnedGenerationJobs(ctx, userID)
	} else {
		rows, err = uc.Store.ListOwnedGenerationJobsBefore(ctx, userID, *cursor)
	}
	if ctx.Err() != nil || err != nil {
		return empty, domain.ErrGenerationJobStore
	}
	page := domain.GenerationJobPage{GenerationJobs: rows}
	if page.GenerationJobs == nil {
		page.GenerationJobs = []domain.GenerationJobSummary{}
	}
	if len(rows) > domain.GenerationJobPageSize {
		page.GenerationJobs = rows[:domain.GenerationJobPageSize]
		next := domain.EncodeGenerationJobCursor(page.GenerationJobs[domain.GenerationJobPageSize-1])
		page.NextCursor = &next
	}
	return page, nil
}
