package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/Xausdorf/mattermost-poll/internal/domain"
)

var (
	ErrInvalidUserID       = errors.New("invalid user id")
	ErrUserIsNotPollAuthor = errors.New("user is not poll author")
	ErrPollNotFound        = errors.New("poll not found")
	ErrPollIsNotActive     = errors.New("poll is not active")
	ErrAnswerNotFound      = errors.New("answer not found")
	ErrAnswerAlreadyExists = errors.New("answer already exists")
)

type PollRepository interface {
	Save(ctx context.Context, poll *domain.Poll) error
	UpdateByID(ctx context.Context, id int64, updateFn func(poll *domain.Poll) error) error
	GetByID(ctx context.Context, id int64) (*domain.Poll, error)
	DeleteByID(ctx context.Context, id int64) error
}

type AnswerRepository interface {
	Save(ctx context.Context, answer *domain.Answer) error
	GetByUserAndPoll(ctx context.Context, userID string, pollID int64) (*domain.Answer, error)
	DeleteByUserAndPoll(ctx context.Context, userID string, pollID int64) error
	DeleteByPoll(ctx context.Context, pollID int64) error
}

type Poll struct {
	pollRepo   PollRepository
	answerRepo AnswerRepository
}

func NewPoll(pollRepo PollRepository, answerRepo AnswerRepository) *Poll {
	return &Poll{
		pollRepo:   pollRepo,
		answerRepo: answerRepo,
	}
}

func (p *Poll) CreatePoll(ctx context.Context, poll *domain.Poll) error {
	return p.pollRepo.Save(ctx, poll)
}

func (p *Poll) AddAnswer(ctx context.Context, answer *domain.Answer) error {
	if err := validateAnswer(answer); err != nil {
		return err
	}

	if ok, err := p.isPollActive(ctx, answer.PollID); err != nil {
		return err
	} else if !ok {
		return ErrPollIsNotActive
	}

	// TODO combine into a single transaction
	if err := p.answerRepo.Save(ctx, answer); err != nil {
		return fmt.Errorf("could not save answer: %w", err)
	}
	if err := p.pollRepo.UpdateByID(ctx, answer.PollID, func(poll *domain.Poll) error {
		if len(poll.Options) <= answer.Vote {
			return fmt.Errorf("there is no such option in poll")
		}
		poll.Options[answer.Vote].Votes++
		return nil
	}); err != nil {
		return fmt.Errorf("could not update poll: %w", err)
	}
	return nil
}

func (p *Poll) GetPollResultsByID(ctx context.Context, id int64) ([]domain.PollOption, error) {
	poll, err := p.pollRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve poll: %w", err)
	}
	return poll.Options, nil
}

func (p *Poll) ClosePollByID(ctx context.Context, id int64, senderID string) error {
	return p.pollRepo.UpdateByID(ctx, id, func(poll *domain.Poll) error {
		if poll.Author != senderID {
			return ErrUserIsNotPollAuthor
		}
		poll.IsActive = false
		return nil
	})
}

func (p *Poll) DeletePollByID(ctx context.Context, id int64, senderID string) error {
	poll, err := p.pollRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("could not retrieve poll: %w", err)
	}

	if poll.Author != senderID {
		return ErrUserIsNotPollAuthor
	}

	if err := p.pollRepo.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("could not delete poll: %w", err)
	}
	// no need for a transaction, records can be deleted manually
	if err := p.answerRepo.DeleteByPoll(ctx, id); err != nil {
		return fmt.Errorf("could not delete poll answers: %w", err)
	}
	return nil
}

func (p *Poll) isPollActive(ctx context.Context, pollID int64) (bool, error) {
	poll, err := p.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		return false, fmt.Errorf("could not retrieve poll: %w", err)
	}
	return poll.IsActive, nil
}

func validateAnswer(answer *domain.Answer) error {
	if answer == nil {
		return fmt.Errorf("answer is nil")
	}
	if answer.UserID == "" {
		return ErrInvalidUserID
	}
	return nil
}
