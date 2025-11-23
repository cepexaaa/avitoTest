package main

import (
	"avito-test-task/internal/api"
	"avito-test-task/internal/config"
	"avito-test-task/internal/handler"
	"avito-test-task/internal/repository"
	pullrequest "avito-test-task/internal/repository/pull_request"
	"avito-test-task/internal/repository/team"
	"avito-test-task/internal/repository/user"
	"avito-test-task/internal/usecase"
	"log"
	"net/http"
)

func main() {
	cfg := config.Load()

	repo, err := repository.NewPostgresRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	db := repo.DB()

	userRepo := user.NewUserRepository(db)
	teamRepo := team.NewTeamRepository(db)
	prRepo := pullrequest.NewPRRepository(db)

	userUC := usecase.NewUserUseCase(*userRepo)
	teamUC := usecase.NewTeamUseCase(*teamRepo, *userRepo)
	prUC := usecase.NewPRUseCase(*prRepo, *userRepo, *teamRepo)

	service := handler.NewServerHandler(teamUC, userUC, prUC)

	strictHandler := api.NewStrictHandler(service, nil)

	router := api.Handler(strictHandler)

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
