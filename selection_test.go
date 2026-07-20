package main

import "testing"

func bounds(s *Selection) [4]int {
	sr, sc, er, ec := s.Bounds()
	return [4]int{sr, sc, er, ec}
}

func TestDragBackwardsKeepsAnchor(t *testing.T) {
	var s Selection
	s.Begin(10, 5)
	s.Extend(5, 2)
	s.End()
	if got, want := bounds(&s), [4]int{5, 2, 10, 5}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
	if !s.Active {
		t.Fatal("selection should be active")
	}
}

func TestRightClickExtendAfterBackwardsDrag(t *testing.T) {
	var s Selection
	s.Begin(10, 5) // drag up from row 10 to row 5
	s.Extend(5, 2)
	s.End()

	s.Extend(20, 0) // right click below the selection
	s.End()

	if got, want := bounds(&s), [4]int{5, 2, 20, 0}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestRightClickExtendBeforeSelection(t *testing.T) {
	var s Selection
	s.Begin(5, 0)
	s.Extend(10, 4)
	s.End()

	s.Extend(2, 3) // right click above the selection
	s.End()

	if got, want := bounds(&s), [4]int{2, 3, 10, 4}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestRightClickInsideShrinksNearestEnd(t *testing.T) {
	var s Selection
	s.Begin(5, 0)
	s.Extend(15, 0)
	s.End()

	s.Extend(13, 2) // nearer the end, so the end moves
	s.End()
	if got, want := bounds(&s), [4]int{5, 0, 13, 2}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	s.Extend(6, 1) // nearer the start, so the start moves
	s.End()
	if got, want := bounds(&s), [4]int{6, 1, 13, 2}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestRightClickWithNoSelectionStartsOne(t *testing.T) {
	var s Selection
	s.Extend(3, 4)
	s.End()
	if s.Active {
		t.Fatal("empty selection should not be active")
	}
}

func TestRepeatedRightClicksKeepFarEndPinned(t *testing.T) {
	var s Selection
	s.Begin(5, 0)
	s.Extend(6, 0)
	s.End()

	for _, row := range []int{9, 12, 20} {
		s.Extend(row, 3)
		s.End()
		if got, want := bounds(&s), [4]int{5, 0, row, 3}; got != want {
			t.Fatalf("after extend to %d: bounds = %v, want %v", row, got, want)
		}
	}
}

func TestWordDragUnionsWholeWords(t *testing.T) {
	var s Selection
	s.BeginRange(2, 4, 2, 9) // double clicked a word
	s.ExtendRange(2, 20, 2, 26)
	s.End()
	if got, want := bounds(&s), [4]int{2, 4, 2, 26}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}

	// dragging back before the anchor word keeps the whole anchor word selected
	s.Selecting = true
	s.ExtendRange(1, 0, 1, 3)
	s.End()
	if got, want := bounds(&s), [4]int{1, 0, 2, 9}; got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
}

func TestClickCollapsesSelection(t *testing.T) {
	var s Selection
	s.Begin(5, 0)
	s.Extend(8, 2)
	s.End()

	s.Begin(3, 1)
	s.End()
	if s.Active {
		t.Fatal("a click without a drag should clear the selection")
	}
}
