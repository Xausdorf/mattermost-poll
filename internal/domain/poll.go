package domain

import "github.com/google/uuid"

// Poll - structure for storing information about poll.
type Poll struct {
	ID       string
	Question string
	Options  []PollOption
	IsActive bool
	// Author - ID of poll's author.
	Author string
}

// PollOption - structure for storing poll's option and voters count.
type PollOption struct {
	Text string
	// Votes - count of users, who voted for this option.
	Votes int
}

// Answer - structure for connecting the user and his vote in the poll.
type Answer struct {
	UserID string
	PollID string
	// Vote - option number in the list of poll's options.
	Vote int
}

func NewPoll(question string, options []PollOption, author string) *Poll {
	return &Poll{
		ID:       uuid.NewString(),
		Question: question,
		Options:  options,
		IsActive: true,
		Author:   author,
	}
}
