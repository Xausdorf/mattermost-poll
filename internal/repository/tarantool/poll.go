package tarantool

import (
	"context"
	"fmt"
	"sync"

	"github.com/Xausdorf/mattermost-poll/internal/domain"
	"github.com/Xausdorf/mattermost-poll/internal/usecase"
	"github.com/tarantool/go-tarantool/v2"
)

const (
	pollSpace = "polls"
)

type PollRepository struct {
	conn *tarantool.Connection
	mu   sync.Mutex
}

func NewPollRepository(conn *tarantool.Connection) *PollRepository {
	return &PollRepository{
		conn: conn,
	}
}

func (r *PollRepository) Save(ctx context.Context, poll *domain.Poll) error {
	_, err := r.conn.Do(
		tarantool.NewInsertRequest(pollSpace).
			Context(ctx).
			Tuple(NewPollModel(poll)),
	).Get()
	return err
}

func (r *PollRepository) GetByID(ctx context.Context, id string) (*domain.Poll, error) {
	var res []PollModel
	if err := r.conn.Do(
		tarantool.NewSelectRequest(pollSpace).
			Context(ctx).
			Index("primary").
			Limit(1).
			Key(tarantool.StringKey{S: id}),
	).GetTyped(&res); err != nil {
		return nil, fmt.Errorf("could not select typed poll in tarantool: %w", err)
	}
	if len(res) == 0 {
		return nil, usecase.ErrPollNotFound
	}
	return res[0].ToPoll(), nil
}

func (r *PollRepository) UpdateByID(ctx context.Context, id string, updateFn func(poll *domain.Poll) error) error {
	r.mu.Lock()
	poll, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err = updateFn(poll); err != nil {
		return fmt.Errorf("could not update poll: %w", err)
	}
	if _, err = r.conn.Do(
		tarantool.NewReplaceRequest(pollSpace).
			Context(ctx).
			Tuple(NewPollModel(poll)),
	).Get(); err != nil {
		return fmt.Errorf("could not replace in tarantool: %w", err)
	}
	r.mu.Unlock()
	return nil
}

func (r *PollRepository) DeleteByID(ctx context.Context, id string) error {
	_, err := r.conn.Do(
		tarantool.NewDeleteRequest(pollSpace).
			Context(ctx).
			Index("primary").
			Key(tarantool.StringKey{S: id}),
	).Get()
	return err
}
