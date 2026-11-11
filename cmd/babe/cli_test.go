package main

import (
	"reflect"
	"testing"
)

func TestReorderFlags(t *testing.T) {
	got := reorderFlags(
		[]string{"app.apk", "-s", "R58M1234", "-3"},
		map[string]bool{"-3": true},
	)
	want := []string{"-s", "R58M1234", "-3", "app.apk"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags() = %v, want %v", got, want)
	}
}
