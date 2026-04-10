package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	scheduledomain "example.com/taskservice/internal/domain/schedule"
	taskdomain "example.com/taskservice/internal/domain/task"
	scheduleusecase "example.com/taskservice/internal/usecase/schedule"
)

// scheduleRequestDTO is the request body shared by POST and PUT schedule endpoints.
// Params is kept as raw JSON so the usecase layer can validate it by kind.
type scheduleRequestDTO struct {
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	DefaultStatus taskdomain.Status   `json:"default_status"`
	Kind          scheduledomain.Kind `json:"kind"`
	Params        json.RawMessage     `json:"params"`
	StartDate     string              `json:"start_date"` // YYYY-MM-DD
	EndDate       *string             `json:"end_date"`   // YYYY-MM-DD or null
	Timezone      string              `json:"timezone"`
}

type scheduleDTO struct {
	ID            int64               `json:"id"`
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	DefaultStatus taskdomain.Status   `json:"default_status"`
	Kind          scheduledomain.Kind `json:"kind"`
	Params        json.RawMessage     `json:"params"`
	StartDate     string              `json:"start_date"`
	EndDate       *string             `json:"end_date,omitempty"`
	Timezone      string              `json:"timezone"`
	Active        bool                `json:"active"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

func newScheduleDTO(s *scheduledomain.Schedule) scheduleDTO {
	dto := scheduleDTO{
		ID:            s.ID,
		Title:         s.Title,
		Description:   s.Description,
		DefaultStatus: s.DefaultStatus,
		Kind:          s.Kind,
		Params:        s.Params,
		StartDate:     s.StartDate.UTC().Format("2006-01-02"),
		Timezone:      s.Timezone,
		Active:        s.Active,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
	if s.EndDate != nil {
		formatted := s.EndDate.UTC().Format("2006-01-02")
		dto.EndDate = &formatted
	}
	return dto
}

func (dto *scheduleRequestDTO) toCreateInput() (scheduleusecase.CreateInput, error) {
	startDate, err := time.Parse("2006-01-02", dto.StartDate)
	if err != nil {
		return scheduleusecase.CreateInput{}, fmt.Errorf("start_date must be YYYY-MM-DD")
	}

	input := scheduleusecase.CreateInput{
		Title:         dto.Title,
		Description:   dto.Description,
		DefaultStatus: dto.DefaultStatus,
		Kind:          dto.Kind,
		RawParams:     dto.Params,
		StartDate:     startDate,
		Timezone:      dto.Timezone,
	}

	if dto.EndDate != nil {
		endDate, err := time.Parse("2006-01-02", *dto.EndDate)
		if err != nil {
			return scheduleusecase.CreateInput{}, fmt.Errorf("end_date must be YYYY-MM-DD")
		}
		input.EndDate = &endDate
	}

	return input, nil
}

func (dto *scheduleRequestDTO) toUpdateInput() (scheduleusecase.UpdateInput, error) {
	startDate, err := time.Parse("2006-01-02", dto.StartDate)
	if err != nil {
		return scheduleusecase.UpdateInput{}, fmt.Errorf("start_date must be YYYY-MM-DD")
	}

	input := scheduleusecase.UpdateInput{
		Title:         dto.Title,
		Description:   dto.Description,
		DefaultStatus: dto.DefaultStatus,
		Kind:          dto.Kind,
		RawParams:     dto.Params,
		StartDate:     startDate,
		Timezone:      dto.Timezone,
	}

	if dto.EndDate != nil {
		endDate, err := time.Parse("2006-01-02", *dto.EndDate)
		if err != nil {
			return scheduleusecase.UpdateInput{}, fmt.Errorf("end_date must be YYYY-MM-DD")
		}
		input.EndDate = &endDate
	}

	return input, nil
}
