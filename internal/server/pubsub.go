package server

import (
	"regexp"
	"sync"
	"time"
)

type Message struct {
	Topic Topic
	Log   Log
}

type Log struct {
	Time time.Time `json:"time"`
	Raw  string    `json:"raw"`
}

type Topic struct {
	App  string `json:"app"`
	Proc string `json:"proc"`
	Host string `json:"host"`
}

type TopicMatcher struct {
	AppPattern  string
	ProcPattern string
	HostPattern string
}

func (matcher TopicMatcher) Matches(topic Topic) bool {
	if !matches(matcher.AppPattern, topic.App) {
		return false
	}
	if !matches(matcher.ProcPattern, topic.Proc) {
		return false
	}
	return true
}

func matches(pattern, s string) bool {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return regex.MatchString(s)
}

type Subscription struct {
	topics TopicMatcher
	msgs   chan Message
}

func (sub *Subscription) Messages() <-chan Message {
	return sub.msgs
}

type Broker struct {
	mu            sync.RWMutex
	subscriptions []*Subscription
}

// Notify sends msg to all subscribers of topic.
func (b *Broker) Notify(l Log, t Topic) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscriptions {
		if sub.topics.Matches(t) {
			sub.msgs <- Message{Log: l, Topic: t}
		}
	}
}

// Subscribe returns a Subscription that provides messages published to matching topics.
func (b *Broker) Subscribe(topics TopicMatcher) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscription := &Subscription{
		topics: topics,
		msgs:   make(chan Message),
	}

	b.subscriptions = append(b.subscriptions, subscription)

	return subscription
}

// Unsubscribe stops sub from receiving further messages.
func (b *Broker) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	close(sub.msgs)
	for i, _ := range b.subscriptions {
		if b.subscriptions[i] == sub {
			b.subscriptions = append(b.subscriptions[:i], b.subscriptions[i+1:]...)
			return
		}
	}
}
