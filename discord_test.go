package main

import (
	"strings"
	"testing"
)

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantMsgs []string
	}{
		{
			name:    "short message",
			content: "hello world",
			wantLen: 1,
			wantMsgs: []string{"hello world"},
		},
		{
			name:    "long message",
			content: strings.Repeat("a", MaxDiscordMessageLength+1),
			wantLen: 2,
			wantMsgs: []string{strings.Repeat("a", MaxDiscordMessageLength), "a\n"},
		},
		{
			name:    "empty message",
			content: "",
			wantLen: 1,
			wantMsgs: []string{""},
		},
		{
			name:    "exact length message",
			content: strings.Repeat("a", MaxDiscordMessageLength),
			wantLen: 1,
			wantMsgs: []string{strings.Repeat("a", MaxDiscordMessageLength)},
		},
	}

	ds := &DiscordSender{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := ds.splitMessage(tt.content)
			if len(msgs) != tt.wantLen {
				t.Errorf("splitMessage() len = %v, want %v", len(msgs), tt.wantLen)
			}

			// A simple way to compare, could be improved with a deep equal
			if len(msgs) == len(tt.wantMsgs) {
				for i, msg := range msgs {
					if msg != tt.wantMsgs[i] {
						t.Errorf("splitMessage() msg[%d] = %v, want %v", i, msg, tt.wantMsgs[i])
					}
				}
			}
		})
	}
}