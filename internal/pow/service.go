package pow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultMaxActiveChallenges is the default limit for active challenges
	DefaultMaxActiveChallenges = 100000
	// ChallengeRandomBytesSize is the size of random bytes in challenge
	ChallengeRandomBytesSize = 16
)

// Service defines the interface for PoW operations
type Service interface {
	GenerateChallenge() (string, error)
	VerifyProof(challenge, nonce string) (bool, error)
	InvalidateChallenge(challenge string)
	SolveChallenge(ctx context.Context, challenge string, difficulty int) (string, error)
	GetDifficulty() int
}

// SHA256HashcashService implements PoW using SHA256 Hashcash algorithm
type SHA256HashcashService struct {
	difficulty          int
	challengeTTL        time.Duration
	maxActiveChallenges int
	activeChallenges    map[string]time.Time // map[challenge]timestamp for replay attack prevention
	mu                  sync.RWMutex         // Protects activeChallenges map
}

// NewSHA256HashcashService creates a new PoW service
func NewSHA256HashcashService(difficulty int, challengeTTL time.Duration) *SHA256HashcashService {
	return NewSHA256HashcashServiceWithLimit(difficulty, challengeTTL, DefaultMaxActiveChallenges)
}

// NewSHA256HashcashServiceWithLimit creates a new PoW service with custom max challenges limit
func NewSHA256HashcashServiceWithLimit(difficulty int, challengeTTL time.Duration, maxActiveChallenges int) *SHA256HashcashService {
	s := &SHA256HashcashService{
		difficulty:          difficulty,
		challengeTTL:        challengeTTL,
		maxActiveChallenges: maxActiveChallenges,
		activeChallenges:    make(map[string]time.Time),
	}

	// Start cleanup goroutine for expired challenges only if TTL is positive
	// (client doesn't need cleanup as it doesn't generate challenges)
	if challengeTTL > 0 {
		go s.cleanupExpiredChallenges()
	}

	return s
}

// GenerateChallenge generates a new unique challenge
func (s *SHA256HashcashService) GenerateChallenge() (string, error) {
	// Generate random bytes
	randomBytes := make([]byte, ChallengeRandomBytesSize)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create challenge: timestamp + random hex string
	timestamp := time.Now().Unix()
	challenge := fmt.Sprintf("%d:%s", timestamp, hex.EncodeToString(randomBytes))

	// Store challenge with timestamp for replay attack prevention
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we've reached the limit of active challenges
	if s.maxActiveChallenges > 0 && len(s.activeChallenges) >= s.maxActiveChallenges {
		return "", fmt.Errorf("maximum active challenges limit reached (%d)", s.maxActiveChallenges)
	}

	s.activeChallenges[challenge] = time.Now()

	return challenge, nil
}

// VerifyProof verifies that the nonce solves the challenge
func (s *SHA256HashcashService) VerifyProof(challenge, nonce string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if challenge exists and is not expired
	timestamp, exists := s.activeChallenges[challenge]
	if !exists {
		return false, fmt.Errorf("challenge not found or already used")
	}

	// Check if challenge is expired
	if time.Since(timestamp) > s.challengeTTL {
		delete(s.activeChallenges, challenge)
		return false, fmt.Errorf("challenge expired")
	}

	// Compute hash
	data := challenge + nonce
	hash := sha256.Sum256([]byte(data))

	// Check if hash has required number of leading zeros
	if !s.hasLeadingZeros(hash[:], s.difficulty) {
		// SECURITY: Remove invalid proof attempt to prevent memory exhaustion
		delete(s.activeChallenges, challenge)
		return false, nil
	}

	// Remove challenge to prevent replay attacks
	delete(s.activeChallenges, challenge)

	return true, nil
}

// InvalidateChallenge removes a challenge from the active set
// This should be called when a connection fails after challenge generation
// to prevent memory exhaustion attacks
func (s *SHA256HashcashService) InvalidateChallenge(challenge string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeChallenges, challenge)
}

// SolveChallenge finds a nonce that solves the challenge
func (s *SHA256HashcashService) SolveChallenge(ctx context.Context, challenge string, difficulty int) (string, error) {
	nonce := 0

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			nonceStr := strconv.Itoa(nonce)
			data := challenge + nonceStr
			hash := sha256.Sum256([]byte(data))

			if s.hasLeadingZeros(hash[:], difficulty) {
				return nonceStr, nil
			}

			nonce++
		}
	}
}

// GetDifficulty returns the current difficulty level
func (s *SHA256HashcashService) GetDifficulty() int {
	return s.difficulty
}

// hasLeadingZeros checks if hash has required number of leading zero bytes
func (s *SHA256HashcashService) hasLeadingZeros(hash []byte, difficulty int) bool {
	if difficulty > len(hash) {
		return false
	}

	for i := 0; i < difficulty; i++ {
		if hash[i] != 0x00 {
			return false
		}
	}

	return true
}

// cleanupExpiredChallenges periodically removes expired challenges
func (s *SHA256HashcashService) cleanupExpiredChallenges() {
	ticker := time.NewTicker(s.challengeTTL / 2)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		s.mu.Lock()
		for challenge, timestamp := range s.activeChallenges {
			if now.Sub(timestamp) > s.challengeTTL {
				delete(s.activeChallenges, challenge)
			}
		}
		s.mu.Unlock()
	}
}
