package people_test

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/app/people"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

type mockPersonRepo struct {
	data map[string]*domain.Person
}

func newMockPersonRepo() *mockPersonRepo {
	return &mockPersonRepo{data: make(map[string]*domain.Person)}
}

func (m *mockPersonRepo) Get(_ context.Context, id string) (*domain.Person, error) {
	p, ok := m.data[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return p, nil
}

func (m *mockPersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	res := make([]*domain.Person, 0, len(m.data))
	for _, p := range m.data {
		res = append(res, p)
	}
	return res, len(res), nil
}

func (m *mockPersonRepo) Save(_ context.Context, p *domain.Person) error {
	if p.ID == "" {
		p.ID = fmt.Sprintf("person-%d", len(m.data)+1)
	}
	m.data[p.ID] = p
	return nil
}

func (m *mockPersonRepo) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func TestCreatePerson_Valid(t *testing.T) {
	repo := newMockPersonRepo()
	svc := people.New(repo)

	p := &domain.Person{Name: "Jane Doe", Aliases: []string{"JD"}}
	if err := svc.CreatePerson(context.Background(), p); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if p.ID == "" {
		t.Error("ID should be set after create")
	}
	if p.SortName != "Jane Doe" {
		t.Errorf("SortName = %q, want same as Name", p.SortName)
	}
	if p.MonitorMode != domain.MonitorAll {
		t.Errorf("MonitorMode = %q, want all", p.MonitorMode)
	}
}

func TestCreatePerson_EmptyName(t *testing.T) {
	svc := people.New(newMockPersonRepo())
	err := svc.CreatePerson(context.Background(), &domain.Person{})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for empty name, got %v", err)
	}
}

func TestCreatePerson_PreservesSortName(t *testing.T) {
	svc := people.New(newMockPersonRepo())
	p := &domain.Person{Name: "Jane Doe", SortName: "Doe, Jane"}
	if err := svc.CreatePerson(context.Background(), p); err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}
	if p.SortName != "Doe, Jane" {
		t.Errorf("SortName = %q, want preserved Doe, Jane", p.SortName)
	}
}

func TestGetPerson_NotFound(t *testing.T) {
	svc := people.New(newMockPersonRepo())
	_, err := svc.GetPerson(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeletePerson_NotFound(t *testing.T) {
	svc := people.New(newMockPersonRepo())
	err := svc.DeletePerson(context.Background(), "nonexistent")
	if !errs.IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSavePerson_EmptyName(t *testing.T) {
	svc := people.New(newMockPersonRepo())
	err := svc.SavePerson(context.Background(), &domain.Person{ID: "p1"})
	if err == nil || !errs.IsValidation(err) {
		t.Errorf("expected ValidationError for empty name, got %v", err)
	}
}

func TestListPeople(t *testing.T) {
	repo := newMockPersonRepo()
	svc := people.New(repo)
	ctx := context.Background()

	for _, name := range []string{"Alice", "Bob", "Carol"} {
		svc.CreatePerson(ctx, &domain.Person{Name: name}) //nolint:errcheck
	}

	list, total, err := svc.ListPeople(ctx, ports.PersonFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
}
