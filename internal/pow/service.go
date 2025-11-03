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

// Service defines the interface for PoW operations
type Service interface {
	GenerateChallenge() (string, error)
	VerifyProof(challenge, nonce string) (bool, error)
	SolveChallenge(ctx context.Context, challenge string, difficulty int) (string, error)
	GetDifficulty() int
}

// SHA256HashcashService implements PoW using SHA256 Hashcash algorithm
type SHA256HashcashService struct {
	difficulty       int
	challengeTTL     time.Duration
	activeChallenges sync.Map // map[challenge]timestamp for replay attack prevention
	mu               sync.Mutex
}

// NewSHA256HashcashService creates a new PoW service
func NewSHA256HashcashService(difficulty int, challengeTTL time.Duration) *SHA256HashcashService {
	s := &SHA256HashcashService{
		difficulty:   difficulty,
		challengeTTL: challengeTTL,
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
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create challenge: timestamp + random hex string
	timestamp := time.Now().Unix()
	challenge := fmt.Sprintf("%d:%s", timestamp, hex.EncodeToString(randomBytes))

	// Store challenge with timestamp for replay attack prevention
	s.activeChallenges.Store(challenge, time.Now())

	return challenge, nil
}

// VerifyProof verifies that the nonce solves the challenge
func (s *SHA256HashcashService) VerifyProof(challenge, nonce string) (bool, error) {
	// Check if challenge exists and is not expired
	value, exists := s.activeChallenges.Load(challenge)
	if !exists {
		return false, fmt.Errorf("challenge not found or already used")
	}

	timestamp, ok := value.(time.Time)
	if !ok {
		return false, fmt.Errorf("invalid challenge timestamp")
	}

	// Check if challenge is expired
	if time.Since(timestamp) > s.challengeTTL {
		s.activeChallenges.Delete(challenge)
		return false, fmt.Errorf("challenge expired")
	}

	// Compute hash
	data := challenge + nonce
	hash := sha256.Sum256([]byte(data))

	// Check if hash has required number of leading zeros
	if !s.hasLeadingZeros(hash[:], s.difficulty) {
		return false, nil
	}

	// Remove challenge to prevent replay attacks
	s.activeChallenges.Delete(challenge)

	return true, nil
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
		s.activeChallenges.Range(func(key, value interface{}) bool {
			timestamp, ok := value.(time.Time)
			if !ok {
				s.activeChallenges.Delete(key)
				return true
			}

			if now.Sub(timestamp) > s.challengeTTL {
				s.activeChallenges.Delete(key)
			}

			return true
		})
	}
}
