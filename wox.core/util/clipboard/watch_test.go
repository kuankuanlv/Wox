package clipboard

import (
	"testing"
)

// fakeClipboardEdge models the contract every platform change detector shares: a
// change is reported exactly once, and reading it consumes it. Losing that single
// report is what makes a missed clipboard entry unrecoverable.
type fakeClipboardEdge struct {
	pending bool
}

func (e *fakeClipboardEdge) detect() bool {
	if !e.pending {
		return false
	}
	e.pending = false
	return true
}

// installFakeClipboardEdge redirects change detection for one test.
func installFakeClipboardEdge(t *testing.T) *fakeClipboardEdge {
	t.Helper()
	edge := &fakeClipboardEdge{}
	previousDetect := detectClipboardChange
	detectClipboardChange = edge.detect
	t.Cleanup(func() {
		detectClipboardChange = previousDetect
	})
	return edge
}

// TestClaimExternalChangeConsumesEdge covers the edge contract: the first caller
// after a clipboard change sees it exactly once, and only it does. Wox's own
// writes are not excluded, so every copy (including Wox-made ones) reaches the
// clipboard history.
func TestClaimExternalChangeConsumesEdge(t *testing.T) {
	edge := installFakeClipboardEdge(t)

	edge.pending = true
	if !claimExternalChange() {
		t.Fatal("a pending change was not claimed")
	}
	if edge.pending {
		t.Fatal("the edge was not consumed by claim")
	}
	if claimExternalChange() {
		t.Fatal("a consumed edge was reported again")
	}
}
