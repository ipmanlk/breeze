package app

import (
	"reflect"
	"testing"

	"ipmanlk/breeze/internal/port"
)

// TestApp_HasSessionRepoField verifies the App struct has a sessionRepo
// field of type port.SessionRepository, ensuring the session cleanup
// wiring is present at the struct level.
func TestApp_HasSessionRepoField(t *testing.T) {
	typ := reflect.TypeOf(App{})
	field, ok := typ.FieldByName("sessionRepo")
	if !ok {
		t.Fatal("App struct missing sessionRepo field")
	}
	if field.Type != reflect.TypeOf((*port.SessionRepository)(nil)).Elem() {
		t.Fatalf("sessionRepo field has type %v, want port.SessionRepository", field.Type)
	}
}
