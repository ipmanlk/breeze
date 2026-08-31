package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
)

var ctx = context.Background()

func TestProcessDueRecurring_Throttle(t *testing.T) {
	tmplRepo := newMockTemplateRepo()
	taskRepo := newMockTaskRepo()
	statusRepo := newMockTaskStatusRepo()
	projRepo := newMockProjectRepo()
	userRepo := newMockUserRepo()

	svc := NewTaskTemplateService(tmplRepo, taskRepo, statusRepo, projRepo, userRepo, nil)

	// Seed a due recurring template
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tmplRepo.Create(nil, &domain.TaskTemplate{
		ID:                "tmpl-1",
		OrgID:             "org-1",
		ProjectID:         "proj-1",
		Name:              "Test recurring",
		Priority:          "medium",
		StatusID:          "status-1",
		RecurrencePattern: "daily",
		NextRunAt:         &past,
		CreatedBy:         "user-1",
	})

	// First call should proceed
	if err := svc.ProcessDueRecurring(ctx); err != nil {
		t.Fatalf("first ProcessDueRecurring: %v", err)
	}

	// Second call immediately after should be throttled
	if err := svc.ProcessDueRecurring(ctx); err != nil {
		t.Fatalf("second ProcessDueRecurring (throttled): %v", err)
	}

	if tmplRepo.listDueCalls != 1 {
		t.Errorf("expected ListDueRecurring to be called once, got %d calls", tmplRepo.listDueCalls)
	}
}

func TestProcessDueRecurring_ConcurrentClaim(t *testing.T) {
	tmplRepo := newMockTemplateRepo()
	taskRepo := newMockTaskRepo()
	statusRepo := newMockTaskStatusRepo()
	projRepo := newMockProjectRepo()
	userRepo := newMockUserRepo()

	// Seed a status so Instantiate can look it up
	statusRepo.Create(nil, &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1", Category: "todo"})

	svc := NewTaskTemplateService(tmplRepo, taskRepo, statusRepo, projRepo, userRepo, nil)

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tmplRepo.Create(nil, &domain.TaskTemplate{
		ID:                "tmpl-1",
		OrgID:             "org-1",
		ProjectID:         "proj-1",
		Name:              "Recurring",
		Priority:          "medium",
		StatusID:          "status-1",
		RecurrencePattern: "daily",
		NextRunAt:         &past,
		CreatedBy:         "user-1",
	})

	// First call: claims and instantiates
	taskCountBefore := len(taskRepo.tasksByID)
	if err := svc.ProcessDueRecurring(ctx); err != nil {
		t.Fatalf("first ProcessDueRecurring: %v", err)
	}
	if len(taskRepo.tasksByID)-taskCountBefore != 1 {
		t.Errorf("expected 1 task created, got %d", len(taskRepo.tasksByID)-taskCountBefore)
	}

	// Verify the template's next_run_at was advanced
	tmpl, _ := tmplRepo.GetByID(ctx, "org-1", "tmpl-1")
	if tmpl.NextRunAt == nil || tmpl.NextRunAt.Equal(past) {
		t.Errorf("expected next_run_at to be advanced past %v, got %v", past, tmpl.NextRunAt)
	}

	// Second call: template is no longer due (next_run_at was advanced), ListDueRecurring returns nothing
	// Reset throttle so it runs again
	svc.mu.Lock()
	svc.lastRecurringRun = time.Time{}
	svc.mu.Unlock()

	taskCountBefore2 := len(taskRepo.tasksByID)
	if err := svc.ProcessDueRecurring(ctx); err != nil {
		t.Fatalf("second ProcessDueRecurring: %v", err)
	}
	if len(taskRepo.tasksByID)-taskCountBefore2 != 0 {
		t.Errorf("expected 0 tasks created on second call (already advanced), got %d", len(taskRepo.tasksByID)-taskCountBefore2)
	}
}

func TestProcessDueRecurring_InstantiateFailureStillAdvances(t *testing.T) {
	tmplRepo := newMockTemplateRepo()
	// Use an error-injecting mock: taskRepo.Create returns error
	taskRepo := &errorTaskRepo{newMockTaskRepo()}
	statusRepo := newMockTaskStatusRepo()
	projRepo := newMockProjectRepo()
	userRepo := newMockUserRepo()

	statusRepo.Create(nil, &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1", Category: "todo"})

	svc := NewTaskTemplateService(tmplRepo, taskRepo, statusRepo, projRepo, userRepo, nil)

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	tmplRepo.Create(nil, &domain.TaskTemplate{
		ID:                "tmpl-1",
		OrgID:             "org-1",
		ProjectID:         "proj-1",
		Name:              "Recurring",
		Priority:          "medium",
		StatusID:          "status-1",
		RecurrencePattern: "daily",
		NextRunAt:         &past,
		CreatedBy:         "user-1",
	})

	// Process: Instantiate will fail (taskRepo.Create returns error)
	// But ClaimDueRecurring should have already advanced next_run_at
	if err := svc.ProcessDueRecurring(ctx); err != nil {
		t.Fatalf("ProcessDueRecurring: %v", err)
	}

	// Verify next_run_at was advanced despite Instantiate failure
	tmpl, _ := tmplRepo.GetByID(ctx, "org-1", "tmpl-1")
	if tmpl.NextRunAt == nil || tmpl.NextRunAt.Equal(past) {
		t.Errorf("expected next_run_at to be advanced past %v despite Instantiate failure, got %v", past, tmpl.NextRunAt)
	}
}

// errorTaskRepo wraps a mockTaskRepo and returns errors on Create.
type errorTaskRepo struct {
	*mockTaskRepo
}

func (e *errorTaskRepo) Create(ctx context.Context, t *domain.Task) error {
	return errors.New("simulated task creation failure")
}

func (e *errorTaskRepo) Update(ctx context.Context, t *domain.Task) error {
	return errors.New("simulated update failure")
}

func TestComputeNextRun_Daily(t *testing.T) {
	from := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	next := computeNextRun("daily", "", from)
	if next == nil {
		t.Fatal("expected non-nil next run")
	}
	expected := from.AddDate(0, 0, 1)
	if !next.Equal(expected) {
		t.Errorf("daily: got %v, want %v", *next, expected)
	}
}

func TestComputeNextRun_Weekly_NoDays(t *testing.T) {
	from := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC) // Thursday
	next := computeNextRun("weekly", "", from)
	if next == nil {
		t.Fatal("expected non-nil next run")
	}
	expected := from.AddDate(0, 0, 7)
	if !next.Equal(expected) {
		t.Errorf("weekly no days: got %v, want %v", *next, expected)
	}
}

func TestComputeNextRun_Weekly_WithDays(t *testing.T) {
	// Monday Jan 19 2026 is the next Monday after Thursday Jan 15
	from := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC) // Thursday
	next := computeNextRun("weekly", "1", from)           // 1 = Monday
	if next == nil {
		t.Fatal("expected non-nil next run")
	}
	expected := time.Date(2026, 1, 19, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("weekly with days: got %v, want %v", *next, expected)
	}
}

func TestComputeNextRun_Monthly_Day15(t *testing.T) {
	from := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	next := computeNextRun("monthly", "15", from)
	if next == nil {
		t.Fatal("expected non-nil next run")
	}
	expected := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("monthly day 15: got %v, want %v", *next, expected)
	}
}

func TestComputeNextRun_Monthly_Day31_ClampedTo28(t *testing.T) {
	// Day 31 should be clamped to 28 to avoid month-length overflow
	from := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	next := computeNextRun("monthly", "31", from)
	if next == nil {
		t.Fatal("expected non-nil next run")
	}
	// February 2026 has 28 days, so clamped to 28
	expected := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("monthly day 31 clamped: got %v, want %v", *next, expected)
	}
}

func TestComputeNextRun_None(t *testing.T) {
	from := time.Now()
	next := computeNextRun("none", "", from)
	if next != nil {
		t.Errorf("none: expected nil, got %v", *next)
	}
}

func TestParseDayNums(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1,3,5", []int{1, 3, 5}},
		{"0", []int{0}},
		{"", []int{}},
		{" 2 , 4 ", []int{2, 4}},
	}
	for _, tt := range tests {
		got := parseDayNums(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseDayNums(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i, v := range got {
			if v != tt.want[i] {
				t.Errorf("parseDayNums(%q)[%d]: got %d, want %d", tt.input, i, v, tt.want[i])
			}
		}
	}
}
