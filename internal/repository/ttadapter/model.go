package ttadapter

import (
	"fmt"

	"github.com/Xausdorf/mattermost-poll/internal/domain"
	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
)

type PollModel struct {
	ID       string
	Question string
	Options  []domain.PollOption
	IsActive bool
	Author   string
}

type AnswerModel struct {
	ID     string
	UserID string
	PollID string
	Vote   int
}

const (
	pollModelFields   = 5
	answerModelFields = 4
)

func NewPollModel(poll *domain.Poll) *PollModel {
	return &PollModel{
		ID:       poll.ID,
		Question: poll.Question,
		Options:  poll.Options,
		IsActive: poll.IsActive,
		Author:   poll.Author,
	}
}

func (p *PollModel) ToPoll() *domain.Poll {
	return &domain.Poll{
		ID:       p.ID,
		Question: p.Question,
		Options:  p.Options,
		IsActive: p.IsActive,
		Author:   p.Author,
	}
}

func (p *PollModel) EncodeMsgpack(e *msgpack.Encoder) error {
	if err := e.EncodeArrayLen(pollModelFields); err != nil {
		return err
	}
	if err := e.EncodeString(p.ID); err != nil {
		return err
	}
	if err := e.EncodeString(p.Question); err != nil {
		return err
	}
	if err := e.Encode(p.Options); err != nil {
		return err
	}
	if err := e.EncodeBool(p.IsActive); err != nil {
		return err
	}
	if err := e.EncodeString(p.Author); err != nil {
		return err
	}
	return nil
}

func (p *PollModel) DecodeMsgpack(d *msgpack.Decoder) error {
	var err error
	var l int
	if l, err = d.DecodeArrayLen(); err != nil {
		return err
	}
	if l != pollModelFields {
		return fmt.Errorf("array len doesn't match: %d", l)
	}
	if p.ID, err = d.DecodeString(); err != nil {
		return err
	}
	if p.Question, err = d.DecodeString(); err != nil {
		return err
	}
	if l, err = d.DecodeArrayLen(); err != nil {
		return err
	}
	p.Options = make([]domain.PollOption, l)
	for i := range l {
		if err = d.Decode(&p.Options[i]); err != nil {
			return err
		}
	}
	if p.IsActive, err = d.DecodeBool(); err != nil {
		return err
	}
	if p.Author, err = d.DecodeString(); err != nil {
		return err
	}
	return nil
}

func NewAnswerModel(answer *domain.Answer) *AnswerModel {
	return &AnswerModel{
		ID:     uuid.NewString(),
		UserID: answer.UserID,
		PollID: answer.PollID,
		Vote:   answer.Vote,
	}
}

func (a *AnswerModel) ToAnswer() *domain.Answer {
	return &domain.Answer{
		UserID: a.UserID,
		PollID: a.PollID,
		Vote:   a.Vote,
	}
}

func (a *AnswerModel) EncodeMsgpack(e *msgpack.Encoder) error {
	if err := e.EncodeArrayLen(answerModelFields); err != nil {
		return err
	}
	if err := e.EncodeString(a.ID); err != nil {
		return err
	}
	if err := e.EncodeString(a.UserID); err != nil {
		return err
	}
	if err := e.EncodeString(a.PollID); err != nil {
		return err
	}
	if err := e.EncodeInt(int64(a.Vote)); err != nil {
		return err
	}
	return nil
}

func (a *AnswerModel) DecodeMsgpack(d *msgpack.Decoder) error {
	var err error
	var l int
	if l, err = d.DecodeArrayLen(); err != nil {
		return err
	}
	if l != answerModelFields {
		return fmt.Errorf("array len doesn't match: %d", l)
	}
	if a.ID, err = d.DecodeString(); err != nil {
		return err
	}
	if a.UserID, err = d.DecodeString(); err != nil {
		return err
	}
	if a.PollID, err = d.DecodeString(); err != nil {
		return err
	}
	if a.Vote, err = d.DecodeInt(); err != nil {
		return err
	}
	return nil
}
