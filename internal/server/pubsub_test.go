package server

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TopicMatcher", func() {
	Describe("Matches", func() {
		It("Should match each topic", func() {
			topics := TopicMatcher{
				AppPattern:  "bar$",
				ProcPattern: "^baz",
			}

			Expect(topics.Matches(Topic{App: "bar", Proc: "baz"})).To(BeTrue())
			Expect(topics.Matches(Topic{App: "open bar", Proc: "baz-ness"})).To(BeTrue())

			topics = TopicMatcher{} // Empty matches everything

			Expect(topics.Matches(Topic{})).To(BeTrue())
			Expect(topics.Matches(Topic{Host: "anything", App: "will", Proc: "do"})).To(BeTrue())
		})
		It("Should not match any topic", func() {
			topics := TopicMatcher{
				HostPattern: "foo.*",
				AppPattern:  "bar$",
				ProcPattern: "^baz",
			}

			Expect(topics.Matches(Topic{Host: "fu not", App: "bar", Proc: "baz"})).To(BeFalse())
			Expect(topics.Matches(Topic{Host: "foo", App: "bart", Proc: "baz"})).To(BeFalse())
			Expect(topics.Matches(Topic{Host: "foo", App: "bar", Proc: "haz baz"})).To(BeFalse())
		})
	})
})

var _ = Describe("Broker", func() {
	It("Should dispatch messages to the right subscribers", func() {
		// defer GinkgoRecover()
		broker := Broker{}

		foosub := broker.Subscribe(TopicMatcher{AppPattern: "^foo$"})
		foos := foosub.Messages()
		barsub := broker.Subscribe(TopicMatcher{AppPattern: "^bar$"})
		bars := barsub.Messages()

		go broker.Notify(Message{Log: "1 foo"}, Topic{App: "foo"})
		go broker.Notify(Message{Log: "1 bar"}, Topic{App: "bar"})
		Expect((<-foos).Log).To(Equal("1 foo"))
		Expect((<-bars).Log).To(Equal("1 bar"))

		go broker.Notify(Message{Log: "2 foo"}, Topic{App: "foo"})
		go broker.Notify(Message{Log: "2 bar"}, Topic{App: "bar"})
		Expect((<-foos).Log).To(Equal("2 foo"))
		Expect((<-bars).Log).To(Equal("2 bar"))

		go broker.Notify(Message{Log: "3 foo"}, Topic{App: "foo"})
		go broker.Notify(Message{Log: "3 bar"}, Topic{App: "bar"})
		Expect((<-foos).Log).To(Equal("3 foo"))
		Expect((<-bars).Log).To(Equal("3 bar"))

		broker.Unsubscribe(foosub)
		broker.Unsubscribe(barsub)

		// Receiving on closed channel results in zero value
		Expect((<-foos)).To(Equal(Message{}))
		Expect((<-bars)).To(Equal(Message{}))
	})
})
