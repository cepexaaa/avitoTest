package usecase

import (
	"avito-test-task/internal/domain"
	"context"
	"testing"

	_ "github.com/lib/pq"
)

func TestUserUseCase_SetUserActivity(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		userID         string
		isActive       bool
		setupData      func()
		expectedActive bool
		expectedError  error
	}{
		{
			name:           "successfully activate inactive user",
			userID:         "user_2",
			isActive:       true,
			setupData:      func() {},
			expectedActive: true,
			expectedError:  nil,
		},
		{
			name:           "successfully deactivate active user",
			userID:         "user_1",
			isActive:       false,
			setupData:      func() {},
			expectedActive: false,
			expectedError:  nil,
		},
		{
			name:           "keep user active when setting same activity",
			userID:         "user_3",
			isActive:       true,
			setupData:      func() {},
			expectedActive: true,
			expectedError:  nil,
		},
		{
			name:           "keep user inactive when setting same activity",
			userID:         "user_2",
			isActive:       false,
			setupData:      func() {},
			expectedActive: false,
			expectedError:  nil,
		},
		{
			name:           "user not found",
			userID:         "non_existent_user",
			isActive:       true,
			setupData:      func() {},
			expectedActive: false,
			expectedError:  domain.ErrUserNotFound,
		},
		{
			name:           "empty user ID",
			userID:         "",
			isActive:       true,
			setupData:      func() {},
			expectedActive: false,
			expectedError:  domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			setupTestData(t)
			tt.setupData()

			result, err := userUseCase.SetUserActivity(ctx, tt.userID, tt.isActive)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.expectedError == nil {
				if result == nil {
					t.Error("Expected user object, got nil")
					return
				}

				if result.IsActive != tt.expectedActive {
					t.Errorf("Expected user active = %t, got %t", tt.expectedActive, result.IsActive)
				}

				dbUser, err := userRepo.FindByID(ctx, tt.userID)
				if err != nil {
					t.Errorf("Failed to verify user in DB: %v", err)
					return
				}

				if dbUser.IsActive != tt.expectedActive {
					t.Errorf("User in DB has active = %t, expected %t", dbUser.IsActive, tt.expectedActive)
				}

				if result.ID != tt.userID {
					t.Errorf("User ID changed: got %s, want %s", result.ID, tt.userID)
				}
				if dbUser.Username == "" {
					t.Error("User username should not be empty")
				}
				if dbUser.TeamID == 0 {
					t.Error("User team ID should not be zero")
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestUserUseCase_SetUserActivity_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		userID      string
		isActive    bool
		setupData   func()
		description string
	}{
		{
			name:     "activate user with special characters in ID",
			userID:   "user-with-dashes",
			isActive: true,
			setupData: func() {

				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ($1, $2, $3, $4)",
					"user-with-dashes", "special_user", 1, false)
			},
			description: "should handle user IDs with special characters",
		},
		{
			name:     "activate user with very long ID",
			userID:   "very_long_user_id_1234567890_abcdefghijklmnopqrstuvwxyz",
			isActive: true,
			setupData: func() {

				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ($1, $2, $3, $4)",
					"very_long_user_id_1234567890_abcdefghijklmnopqrstuvwxyz", "long_user", 1, false)
			},
			description: "should handle very long user IDs",
		},
		{
			name:     "user in different teams",
			userID:   "team2_user",
			isActive: false,
			setupData: func() {

				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ($1, $2, $3, $4)",
					"team2_user", "team2_user", 2, true)
			},
			description: "should handle users from different teams",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			result, err := userUseCase.SetUserActivity(ctx, tt.userID, tt.isActive)

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
				return
			}

			if result == nil {
				t.Errorf("Expected user object for %s, got nil", tt.description)
				return
			}

			if result.IsActive != tt.isActive {
				t.Errorf("For %s: expected active = %t, got %t", tt.description, tt.isActive, result.IsActive)
			}

			dbUser, err := userRepo.FindByID(ctx, tt.userID)
			if err != nil {
				t.Errorf("Failed to verify user in DB for %s: %v", tt.description, err)
				return
			}

			if dbUser.IsActive != tt.isActive {
				t.Errorf("User in DB for %s has active = %t, expected %t", tt.description, dbUser.IsActive, tt.isActive)
			}
		})
	}
}

func TestUserUseCase_SetUserActivity_InvalidValues(t *testing.T) {
	ctx := context.Background()

	invalidTests := []struct {
		name      string
		userID    string
		isActive  bool
		expectErr bool
	}{
		{
			name:      "very long user ID that exceeds limit",
			userID:    string(make([]byte, 1000)),
			isActive:  true,
			expectErr: true,
		},
		{
			name:      "SQL injection attempt in user ID",
			userID:    "user_1'; DROP TABLE users; --",
			isActive:  true,
			expectErr: true,
		},
		{
			name:      "user ID with null bytes",
			userID:    "user\x00withnull",
			isActive:  true,
			expectErr: true,
		},
		{
			name:      "user ID with only spaces",
			userID:    "   ",
			isActive:  true,
			expectErr: true,
		},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)

			result, err := userUseCase.SetUserActivity(ctx, tt.userID, tt.isActive)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error for invalid input, got nil")
				}
				if result != nil {
					t.Error("Expected nil result for invalid input")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid input: %v", err)
				}
			}
		})
	}
}

func TestUserUseCase_SetUserActivity_ConcurrentUpdates(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent activity updates", func(t *testing.T) {
		setupTestData(t)

		userID := "user_1"
		iterations := 10

		errors := make(chan error, iterations)

		for i := 0; i < iterations; i++ {
			go func(index int) {

				active := index%2 == 0
				_, err := userUseCase.SetUserActivity(ctx, userID, active)
				errors <- err
			}(i)
		}

		for i := 0; i < iterations; i++ {
			err := <-errors
			if err != nil {
				t.Errorf("Concurrent update failed: %v", err)
			}
		}

		finalUser, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			t.Errorf("Failed to get final user state: %v", err)
			return
		}

		if finalUser.IsActive != true && finalUser.IsActive != false {
			t.Error("User should have valid activity state after concurrent updates")
		}

		if finalUser.ID != userID {
			t.Errorf("User ID corrupted: got %s, want %s", finalUser.ID, userID)
		}
		if finalUser.Username != "alice" {
			t.Errorf("User username corrupted: got %s, want alice", finalUser.Username)
		}
	})
}
