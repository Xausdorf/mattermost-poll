package ttadapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/Xausdorf/mattermost-poll/internal/domain"
	"github.com/Xausdorf/mattermost-poll/internal/usecase"
	"github.com/tarantool/go-tarantool/v2"
)

const (
	answerSpace = "answers"
)

type AnswerRepository struct {
	conn *tarantool.Connection
}

func NewAnswerRepository(conn *tarantool.Connection) *AnswerRepository {
	return &AnswerRepository{
		conn: conn,
	}
}

func (r *AnswerRepository) Save(ctx context.Context, answer *domain.Answer) error {
	_, err := r.GetByUserAndPoll(ctx, answer.UserID, answer.PollID)
	if err == nil {
		return usecase.ErrAnswerAlreadyExists
	} else if !errors.Is(err, usecase.ErrAnswerNotFound) {
		return err
	}
	_, err = r.conn.Do(
		tarantool.NewInsertRequest(answerSpace).
			Context(ctx).
			Tuple(NewAnswerModel(answer)),
	).Get()
	return err
}

func (r *AnswerRepository) GetByUserAndPoll(ctx context.Context, userID string, pollID string) (*domain.Answer, error) {
	var res []AnswerModel
	if err := r.conn.Do(
		tarantool.NewSelectRequest(pollSpace).
			Context(ctx).
			Index("user_poll").
			Limit(1).
			Key([]interface{}{userID, pollID}),
	).GetTyped(&res); err != nil {
		return nil, fmt.Errorf("could not select typed answer in tarantool: %w", err)
	}
	if len(res) == 0 {
		return nil, usecase.ErrAnswerNotFound
	}
	return res[0].ToAnswer(), nil
}

func (r *AnswerRepository) DeleteByPoll(context.Context, string) error {
	// Do nothing, tarantool does not allow delete by a non-unique key
	return nil
}
