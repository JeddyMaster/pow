package quotes

import (
	"math/rand"
	"sync"
	"time"
)

// Service defines the interface for quotes operations
type Service interface {
	GetRandomQuote() string
}

// InMemoryService implements quotes service with in-memory storage
type InMemoryService struct {
	quotes []string
	rng    *rand.Rand
	mu     sync.Mutex // Protects rng from concurrent access
}

// NewInMemoryService creates a new quotes service
func NewInMemoryService() *InMemoryService {
	return &InMemoryService{
		quotes: []string{
			"The only way to do great work is to love what you do. - Steve Jobs",
			"Innovation distinguishes between a leader and a follower. - Steve Jobs",
			"Life is what happens when you're busy making other plans. - John Lennon",
			"The future belongs to those who believe in the beauty of their dreams. - Eleanor Roosevelt",
			"It is during our darkest moments that we must focus to see the light. - Aristotle",
			"The only impossible journey is the one you never begin. - Tony Robbins",
			"In the middle of difficulty lies opportunity. - Albert Einstein",
			"Life is 10% what happens to you and 90% how you react to it. - Charles R. Swindoll",
			"The best time to plant a tree was 20 years ago. The second best time is now. - Chinese Proverb",
			"Your time is limited, don't waste it living someone else's life. - Steve Jobs",
			"Whether you think you can or you think you can't, you're right. - Henry Ford",
			"The only person you are destined to become is the person you decide to be. - Ralph Waldo Emerson",
			"Go confidently in the direction of your dreams! Live the life you've imagined. - Henry David Thoreau",
			"Everything you've ever wanted is on the other side of fear. - George Addair",
			"Success is not final, failure is not fatal: it is the courage to continue that counts. - Winston Churchill",
			"Hardships often prepare ordinary people for an extraordinary destiny. - C.S. Lewis",
			"Believe you can and you're halfway there. - Theodore Roosevelt",
			"The only limit to our realization of tomorrow will be our doubts of today. - Franklin D. Roosevelt",
			"It does not matter how slowly you go as long as you do not stop. - Confucius",
			"Act as if what you do makes a difference. It does. - William James",
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetRandomQuote returns a random quote from the collection
// This method is safe for concurrent use
func (s *InMemoryService) GetRandomQuote() string {
	if len(s.quotes) == 0 {
		return "No quotes available"
	}

	s.mu.Lock()
	index := s.rng.Intn(len(s.quotes))
	s.mu.Unlock()

	return s.quotes[index]
}
