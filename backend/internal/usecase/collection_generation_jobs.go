package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionGenerationJobStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListOwnedCollectionGenerationJobs(context.Context, string, string) (bool, []domain.GenerationJobSummary, error)
	ListOwnedCollectionGenerationJobsBefore(context.Context, string, string, domain.GenerationJobCursor) (bool, []domain.GenerationJobSummary, error)
}

type CollectionGenerationJobs struct{ Store CollectionGenerationJobStore }

func (uc CollectionGenerationJobs) List(ctx context.Context, digest [32]byte, collectionID, rawQuery string) (domain.GenerationJobPage, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.GenerationJobPage{}, domain.ErrGenerationJobStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return domain.GenerationJobPage{}, err
	}
	if !domain.ValidIdentifier(collectionID) {
		return domain.GenerationJobPage{}, domain.ErrInvalidIdentifier
	}
	cursor, err := generationJobCursor(rawQuery)
	if err != nil {
		return domain.GenerationJobPage{}, err
	}
	var exists bool
	var rows []domain.GenerationJobSummary
	if cursor == nil {
		exists, rows, err = uc.Store.ListOwnedCollectionGenerationJobs(ctx, userID, collectionID)
	} else {
		exists, rows, err = uc.Store.ListOwnedCollectionGenerationJobsBefore(ctx, userID, collectionID, *cursor)
	}
	if ctx.Err() != nil {
		return domain.GenerationJobPage{}, domain.ErrGenerationJobStore
	}
	if err != nil {
		return domain.GenerationJobPage{}, err
	}
	if !exists {
		return domain.GenerationJobPage{}, domain.ErrResourceNotFound
	}
	return generationJobPage(rows), nil
}
