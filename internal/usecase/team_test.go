package usecase

import (
	"avito-test-task/internal/domain"
	"context"
	"testing"

	_ "github.com/lib/pq"
)

func TestTeamUseCase_CreateTeam(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		team          *domain.Team
		expectedError error
		description   string
	}{
		{
			name: "successfully create team with members",
			team: &domain.Team{
				Name: "devops-team",
				Members: []domain.TeamMember{
					{UserID: "dev_user_1", Username: "devops1", IsActive: true},
					{UserID: "dev_user_2", Username: "devops2", IsActive: true},
				},
			},
			expectedError: nil,
			description:   "should create new team with members successfully",
		},
		{
			name: "create team without members",
			team: &domain.Team{
				Name:    "qa-team",
				Members: []domain.TeamMember{},
			},
			expectedError: nil,
			description:   "should create team without members",
		},
		{
			name: "create team with mixed active/inactive members",
			team: &domain.Team{
				Name: "mobile-team",
				Members: []domain.TeamMember{
					{UserID: "mobile_1", Username: "mobile_dev1", IsActive: true},
					{UserID: "mobile_2", Username: "mobile_dev2", IsActive: false},
				},
			},
			expectedError: nil,
			description:   "should create team with mixed member activity status",
		},
		{
			name: "fail to create team with duplicate name",
			team: &domain.Team{
				Name:    "backend-team",
				Members: []domain.TeamMember{},
			},
			expectedError: domain.ErrTeamExists,
			description:   "should fail when team name already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)

			result, err := teamUseCase.CreateTeam(ctx, tt.team)

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
					t.Error("Expected team object, got nil")
					return
				}

				if result.ID == 0 {
					t.Error("Team ID should be set after creation")
				}

				if result.Name != tt.team.Name {
					t.Errorf("Team name mismatch: got %s, want %s", result.Name, tt.team.Name)
				}

				dbTeam, err := teamRepo.FindByName(ctx, tt.team.Name)
				if err != nil {
					t.Errorf("Failed to verify team in DB: %v", err)
					return
				}

				if dbTeam.Name != tt.team.Name {
					t.Errorf("Team in DB has name %s, expected %s", dbTeam.Name, tt.team.Name)
				}

				if len(tt.team.Members) > 0 {

					if len(result.Members) != len(tt.team.Members) {
						t.Errorf("Expected %d members in result, got %d", len(tt.team.Members), len(result.Members))
					}

					_, err := userRepo.FindByTeamID(ctx, result.ID)
					if err != nil {
						t.Errorf("Failed to get team members from DB: %v", err)
						return
					}

					for i, expectedMember := range tt.team.Members {

						if i < len(result.Members) {
							if result.Members[i].UserID != expectedMember.UserID {
								t.Errorf("Member %d UserID mismatch: got %s, want %s", i, result.Members[i].UserID, expectedMember.UserID)
							}
							if result.Members[i].Username != expectedMember.Username {
								t.Errorf("Member %d Username mismatch: got %s, want %s", i, result.Members[i].Username, expectedMember.Username)
							}
							if result.Members[i].IsActive != expectedMember.IsActive {
								t.Errorf("Member %d IsActive mismatch: got %t, want %t", i, result.Members[i].IsActive, expectedMember.IsActive)
							}
						}

						dbUser, err := userRepo.FindByID(ctx, expectedMember.UserID)
						if err != nil {
							t.Errorf("Failed to find user %s in DB: %v", expectedMember.UserID, err)
							continue
						}

						if dbUser.Username != expectedMember.Username {
							t.Errorf("DB user %s username mismatch: got %s, want %s", expectedMember.UserID, dbUser.Username, expectedMember.Username)
						}
						if dbUser.TeamID != result.ID {
							t.Errorf("DB user %s team ID mismatch: got %d, want %d", expectedMember.UserID, dbUser.TeamID, result.ID)
						}
						if dbUser.TeamName != tt.team.Name {
							t.Errorf("DB user %s team name mismatch: got %s, want %s", expectedMember.UserID, dbUser.TeamName, tt.team.Name)
						}
						if dbUser.IsActive != expectedMember.IsActive {
							t.Errorf("DB user %s active status mismatch: got %t, want %t", expectedMember.UserID, dbUser.IsActive, expectedMember.IsActive)
						}
					}
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestTeamUseCase_GetTeam(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		teamName        string
		setupData       func()
		expectedError   error
		expectedMembers int
		description     string
	}{
		{
			name:     "successfully get existing team with members",
			teamName: "backend-team",
			setupData: func() {

			},
			expectedError:   nil,
			expectedMembers: 3,
			description:     "should return team with all members",
		},
		{
			name:     "get team without members",
			teamName: "empty-team",
			setupData: func() {

				testDB.Exec("INSERT INTO teams (name) VALUES ('empty-team')")
			},
			expectedError:   nil,
			expectedMembers: 0,
			description:     "should return team with empty members list",
		},
		{
			name:     "get non-existent team",
			teamName: "non-existent-team",
			setupData: func() {

			},
			expectedError:   domain.ErrTeamNotFound,
			expectedMembers: 0,
			description:     "should return error for non-existent team",
		},
		{
			name:     "get team with mixed active/inactive members",
			teamName: "mixed-team",
			setupData: func() {
				testDB.Exec("INSERT INTO teams (name) VALUES ('mixed-team')")

				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ('mixed_1', 'mixed_user1', (SELECT id FROM teams WHERE name = 'mixed-team'), true)")
				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ('mixed_2', 'mixed_user2', (SELECT id FROM teams WHERE name = 'mixed-team'), false)")
				testDB.Exec("INSERT INTO users (id, username, team_id, is_active) VALUES ('mixed_3', 'mixed_user3', (SELECT id FROM teams WHERE name = 'mixed-team'), true)")
			},
			expectedError:   nil,
			expectedMembers: 3,
			description:     "should return all members regardless of activity status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)
			tt.setupData()

			result, err := teamUseCase.GetTeam(ctx, tt.teamName)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			}

			if tt.expectedError == nil {
				if result == nil {
					t.Error("Expected team object, got nil")
					return
				}

				if result.Name != tt.teamName {
					t.Errorf("Team name mismatch: got %s, want %s", result.Name, tt.teamName)
				}

				if result.ID == 0 {
					t.Error("Team ID should be set")
				}

				if len(result.Members) != tt.expectedMembers {
					t.Errorf("Expected %d members, got %d", tt.expectedMembers, len(result.Members))
				}

				for i, member := range result.Members {
					if member.UserID == "" {
						t.Errorf("Member %d has empty UserID", i)
					}
					if member.Username == "" {
						t.Errorf("Member %d has empty Username", i)
					}
				}

				if tt.expectedMembers > 0 {

					dbUsers, err := userRepo.FindByTeamID(ctx, result.ID)
					if err != nil {
						t.Errorf("Failed to verify members in DB: %v", err)
						return
					}

					if len(dbUsers) != tt.expectedMembers {
						t.Errorf("DB has %d users for team, expected %d", len(dbUsers), tt.expectedMembers)
					}
				}
			} else {

				if result != nil {
					t.Error("Expected nil result when error occurs")
				}
			}
		})
	}
}

func TestTeamUseCase_EdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		teamName      string
		operation     func() error
		expectedError error
		description   string
	}{
		{
			name:     "get team with empty name",
			teamName: "",
			operation: func() error {
				_, err := teamUseCase.GetTeam(ctx, "")
				return err
			},
			expectedError: domain.ErrTeamNotFound,
			description:   "should handle empty team name",
		},
		{
			name:     "create team with very long name",
			teamName: "very-long-team-name-" + string(make([]byte, 200)),
			operation: func() error {
				_, err := teamUseCase.CreateTeam(ctx, &domain.Team{
					Name:    "very-long-team-name-" + string(make([]byte, 200)),
					Members: []domain.TeamMember{},
				})
				return err
			},
			expectedError: domain.ErrTeamExists,
			description:   "should handle very long team names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestData(t)

			err := tt.operation()

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.description, err)
				}
			}
		})
	}
}
