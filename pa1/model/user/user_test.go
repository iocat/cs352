package user

import (
	"sync"
	"testing"

	"github.com/iocat/rutgers-cs352/pa1/model/color"
	"github.com/iocat/rutgers-cs352/pa1/model/message"
)

func TestUserMessage(t *testing.T) {
	var wg sync.WaitGroup
	cases := []struct {
		user     *ConcreteUser
		others   []*ConcreteUser
		messages []string
	}{
		{
			user: New("felix", color.Randomize(), func(m message.Message) {
				defer wg.Done()
				t.Logf("%s", m.String())
			}),
			others: []*ConcreteUser{
				New("iocat", color.Randomize(), nil),
				New("felixinf", color.Randomize(), nil),
				New("thanh", color.Randomize(), nil),
				New("tcn33", color.Randomize(), nil),
				New("link", color.Randomize(), nil),
				New("newme", color.Randomize(), nil),
			},
			messages: []string{
				"hello",
				"what's up?",
				"oh hello there!",
				"hi there, thanh!",
				"this is link, welcome :))",
				"another one?",
			},
		},
	}

	for _, test := range cases {
		for i, other := range test.others {
			wg.Add(1)
			test.user.Receive(other.Message(test.messages[i]))
		}
		test.user.SignOut()
	}
	wg.Wait()

}
