package model

import "testing"

func TestIsValidListSort(t *testing.T) {
	if !IsValidListSort(int(RSortTimeDesc)) {
		t.Fatal("expected list sort to accept time desc")
	}
	if IsValidListSort(int(RSortRelevance)) {
		t.Fatal("expected list sort to reject relevance")
	}
}

func TestIsValidSearchSort(t *testing.T) {
	if !IsValidSearchSort(int(RSortRelevance)) {
		t.Fatal("expected search sort to accept relevance")
	}
	if IsValidSearchSort(int(RSortRelevance) + 1) {
		t.Fatal("expected search sort to reject values above relevance")
	}
	if IsValidSearchSort(-1) {
		t.Fatal("expected search sort to reject negative values")
	}
}
