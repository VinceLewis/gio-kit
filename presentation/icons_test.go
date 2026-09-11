package presentation

import (
	"reflect"
	"testing"
)

func TestSemanticIconKeepsExitAndCloseDistinct(t *testing.T) {
	exit := SemanticIcon("log-out")
	logout := SemanticIcon("logout")
	close := SemanticIcon("close")
	x := SemanticIcon("x")
	if exit == nil || logout == nil || close == nil || x == nil {
		t.Fatal("expected exit and close aliases to resolve")
	}
	if !reflect.DeepEqual(exit, logout) {
		t.Fatal("log-out and logout should resolve to the same icon")
	}
	if !reflect.DeepEqual(close, x) {
		t.Fatal("close and x should resolve to the same icon")
	}
	if reflect.DeepEqual(exit, close) {
		t.Fatal("exit and close must resolve to distinct icons")
	}
}
