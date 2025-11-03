package pow

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

func TestSHA256HashcashService_GenerateChallenge(t *testing.T) {
	service := NewSHA256HashcashService(2, 5*time.Minute)

	challenge, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	if challenge == "" {
		t.Error("Challenge should not be empty")
	}

	// Check format: timestamp:randomhex
	parts := strings.Split(challenge, ":")
	if len(parts) != 2 {
		t.Errorf("Challenge should have format 'timestamp:randomhex', got: %s", challenge)
	}

	// Generate another challenge and ensure it's different
	challenge2, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	if challenge == challenge2 {
		t.Error("Two consecutive challenges should be different")
	}
}

func TestSHA256HashcashService_VerifyProof(t *testing.T) {
	difficulty := 1
	service := NewSHA256HashcashService(difficulty, 5*time.Minute)

	challenge, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	// Solve the challenge
	ctx := context.Background()
	nonce, err := service.SolveChallenge(ctx, challenge, difficulty)
	if err != nil {
		t.Fatalf("SolveChallenge failed: %v", err)
	}

	// Verify the proof
	valid, err := service.VerifyProof(challenge, nonce)
	if err != nil {
		t.Fatalf("VerifyProof failed: %v", err)
	}

	if !valid {
		t.Error("Proof should be valid")
	}
}

func TestSHA256HashcashService_VerifyProof_Invalid(t *testing.T) {
	difficulty := 2
	service := NewSHA256HashcashService(difficulty, 5*time.Minute)

	challenge, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	// Use an invalid nonce
	valid, err := service.VerifyProof(challenge, "invalid_nonce")
	if err != nil {
		t.Fatalf("VerifyProof failed: %v", err)
	}

	if valid {
		t.Error("Proof should be invalid")
	}
}

func TestSHA256HashcashService_VerifyProof_ExpiredChallenge(t *testing.T) {
	difficulty := 1
	service := NewSHA256HashcashService(difficulty, 100*time.Millisecond)

	challenge, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	// Solve the challenge
	ctx := context.Background()
	nonce, err := service.SolveChallenge(ctx, challenge, difficulty)
	if err != nil {
		t.Fatalf("SolveChallenge failed: %v", err)
	}

	// Wait for challenge to expire
	time.Sleep(150 * time.Millisecond)

	// Verify the proof (should fail due to expiration)
	valid, err := service.VerifyProof(challenge, nonce)
	if err == nil {
		t.Error("VerifyProof should return an error for expired challenge")
	}

	if valid {
		t.Error("Proof should be invalid for expired challenge")
	}
}

func TestSHA256HashcashService_VerifyProof_ReplayAttack(t *testing.T) {
	difficulty := 1
	service := NewSHA256HashcashService(difficulty, 5*time.Minute)

	challenge, err := service.GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	// Solve the challenge
	ctx := context.Background()
	nonce, err := service.SolveChallenge(ctx, challenge, difficulty)
	if err != nil {
		t.Fatalf("SolveChallenge failed: %v", err)
	}

	// Verify the proof (should succeed)
	valid, err := service.VerifyProof(challenge, nonce)
	if err != nil {
		t.Fatalf("VerifyProof failed: %v", err)
	}

	if !valid {
		t.Error("Proof should be valid")
	}

	// Try to verify the same proof again (should fail - replay attack prevention)
	valid, err = service.VerifyProof(challenge, nonce)
	if err == nil {
		t.Error("VerifyProof should return an error for reused challenge")
	}

	if valid {
		t.Error("Proof should be invalid for reused challenge")
	}
}

func TestSHA256HashcashService_SolveChallenge(t *testing.T) {
	difficulty := 1
	service := NewSHA256HashcashService(difficulty, 5*time.Minute)

	challenge := "test_challenge"
	ctx := context.Background()

	nonce, err := service.SolveChallenge(ctx, challenge, difficulty)
	if err != nil {
		t.Fatalf("SolveChallenge failed: %v", err)
	}

	// Verify that the solution is correct
	data := challenge + nonce
	hash := sha256.Sum256([]byte(data))

	if !service.hasLeadingZeros(hash[:], difficulty) {
		t.Errorf("Solution does not have %d leading zeros", difficulty)
	}
}

func TestSHA256HashcashService_SolveChallenge_Timeout(t *testing.T) {
	difficulty := 10 // Very high difficulty to ensure timeout
	service := NewSHA256HashcashService(difficulty, 5*time.Minute)

	challenge := "test_challenge"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := service.SolveChallenge(ctx, challenge, difficulty)
	if err == nil {
		t.Error("SolveChallenge should return an error on timeout")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestSHA256HashcashService_hasLeadingZeros(t *testing.T) {
	service := NewSHA256HashcashService(2, 5*time.Minute)

	tests := []struct {
		name       string
		hash       []byte
		difficulty int
		want       bool
	}{
		{
			name:       "1 leading zero",
			hash:       []byte{0x00, 0x01, 0x02},
			difficulty: 1,
			want:       true,
		},
		{
			name:       "2 leading zeros",
			hash:       []byte{0x00, 0x00, 0x01},
			difficulty: 2,
			want:       true,
		},
		{
			name:       "No leading zeros",
			hash:       []byte{0x01, 0x02, 0x03},
			difficulty: 1,
			want:       false,
		},
		{
			name:       "1 leading zero but need 2",
			hash:       []byte{0x00, 0x01, 0x02},
			difficulty: 2,
			want:       false,
		},
		{
			name:       "Difficulty exceeds hash length",
			hash:       []byte{0x00, 0x00},
			difficulty: 3,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.hasLeadingZeros(tt.hash, tt.difficulty)
			if got != tt.want {
				t.Errorf("hasLeadingZeros() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkSolveChallenge_Difficulty1(b *testing.B) {
	service := NewSHA256HashcashService(1, 5*time.Minute)
	challenge := "benchmark_challenge"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.SolveChallenge(ctx, challenge, 1)
		if err != nil {
			b.Fatalf("SolveChallenge failed: %v", err)
		}
	}
}

func BenchmarkSolveChallenge_Difficulty2(b *testing.B) {
	service := NewSHA256HashcashService(2, 5*time.Minute)
	challenge := "benchmark_challenge"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.SolveChallenge(ctx, challenge, 2)
		if err != nil {
			b.Fatalf("SolveChallenge failed: %v", err)
		}
	}
}

func BenchmarkVerifyProof(b *testing.B) {
	service := NewSHA256HashcashService(2, 5*time.Minute)
	challenge := "benchmark_challenge"
	ctx := context.Background()

	// Pre-solve the challenge
	nonce, err := service.SolveChallenge(ctx, challenge, 2)
	if err != nil {
		b.Fatalf("SolveChallenge failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Re-add challenge for each iteration
		service.activeChallenges.Store(challenge, time.Now())
		_, err := service.VerifyProof(challenge, nonce)
		if err != nil {
			b.Fatalf("VerifyProof failed: %v", err)
		}
	}
}
