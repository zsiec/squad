package server

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestBoardDragDropFiresAssignOnIdleAgent drives the SPA drag-and-drop
// wiring through Node and asserts that releasing a ready row over an
// idle agent row produces a POST /api/items/{id}/assign request.
//
// Pre-fix: the dragover handler gates `preventDefault()` on a runtime
// check of `dataTransfer.types`, which is browser-fragile during
// dragenter/dragover; if the type lookup misses, the row is rejected
// as a drop target and the drop event never fires. Post-fix: the
// gate is a module-level drag-state flag set in dragstart, so the
// preventDefault is unconditional once a squad-item drag is live.
//
// Same node-eval harness pattern as TestRenderInsightsLastCallWins
// in insights_race_test.go.
func TestBoardDragDropFiresAssignOnIdleAgent(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	const harness = `
const calls = { fetch: [], handlers: {}, leaveTrace: [] };

globalThis.fetch = async (url, opts) => {
  calls.fetch.push({ url, body: opts && opts.body });
  return { ok: true, status: 200, json: async () => ({ ok: true }) };
};

function makeEl(tag) {
  const el = {
    _tag: tag,
    _handlers: {},
    dataset: {},
    classList: {
      add: () => {}, remove: () => {}, toggle: () => {}, contains: () => false,
    },
    children: [],
    style: {},
    appendChild(c) { this.children.push(c); return c; },
    removeAttribute(k) { delete this.dataset[k.replace(/^data-/, '').replace(/-./g, m => m[1].toUpperCase())]; },
    setAttribute() {},
    addEventListener(type, fn) { this._handlers[type] = fn; },
    removeEventListener() {},
    querySelector: () => makeEl('div'),
    querySelectorAll: () => [],
    contains: () => false,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 100, height: 30 }),
    insertAdjacentElement() {},
  };
  return el;
}

globalThis.document = {
  body: makeEl('body'),
  head: makeEl('head'),
  getElementById: () => makeEl('div'),
  querySelector: () => makeEl('div'),
  querySelectorAll: () => [],
  createElement: makeEl,
  createTextNode: (t) => ({ nodeValue: t }),
  addEventListener: () => {},
  removeEventListener: () => {},
};
globalThis.window = globalThis;
globalThis.NodeFilter = { SHOW_TEXT: 4 };

class FakeDataTransfer {
  constructor() {
    this._data = new Map();
    this._typesArr = [];
  }
  setData(type, value) {
    if (!this._data.has(type)) this._typesArr.push(type);
    this._data.set(type, value);
  }
  getData(type) { return this._data.get(type) || ''; }
  get types() { return [...this._typesArr]; }
  set effectAllowed(_) {} get effectAllowed() { return 'move'; }
  set dropEffect(_) {} get dropEffect() { return 'move'; }
}

const board = await import('./board.js');

const li = makeEl('li');
li.dataset.state = 'idle';
board.wireIdleDrop(li, 'agent-target', 'Target');

// Source-side dragstart populates the dataTransfer. The browser
// shares this dataTransfer across the whole drag lifecycle (the
// destination side reads it during dragenter/dragover/drop).
const dt = new FakeDataTransfer();
dt.setData('text/plain', 'BUG-999');
dt.setData('application/x-squad-item', 'BUG-999');

// Notify the module that a drag is in flight (post-fix path).
if (typeof board.notifyDragStartForTest === 'function') {
  board.notifyDragStartForTest('BUG-999');
}

function fire(type, target) {
  const ev = {
    type,
    target: target || li,
    dataTransfer: dt,
    _defaultPrevented: false,
    preventDefault() { this._defaultPrevented = true; },
  };
  const fn = li._handlers[type];
  if (fn) fn(ev);
  return ev;
}

const enter   = fire('dragenter');
const over    = fire('dragover');
const drop    = fire('drop');

// Drain microtasks so the async assignItemToAgent's fetch lands.
await new Promise((r) => setTimeout(r, 20));

process.stdout.write(JSON.stringify({
  enterPrevented: enter._defaultPrevented,
  overPrevented:  over._defaultPrevented,
  dropPrevented:  drop._defaultPrevented,
  dropTargetSet:  li.dataset.dropTarget === 'true' || li.dataset.dropTarget === undefined,
  fetched:        calls.fetch.map(c => ({ url: c.url, body: c.body })),
}));
`

	cmd := exec.Command("node", "--input-type=module", "--eval", harness)
	cmd.Dir = "web"
	stdout, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node harness failed: %v\nstderr: %s", err, stderr)
	}

	var result struct {
		EnterPrevented bool `json:"enterPrevented"`
		OverPrevented  bool `json:"overPrevented"`
		DropPrevented  bool `json:"dropPrevented"`
		Fetched        []struct {
			URL  string `json:"url"`
			Body string `json:"body"`
		} `json:"fetched"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("decode: %v\nstdout: %s", err, stdout)
	}

	// dragover MUST preventDefault — this is the contract that makes
	// the row a valid drop target. If this is false, the browser
	// silently rejects the drop and the entire feature is broken.
	if !result.OverPrevented {
		t.Errorf("dragover handler must call preventDefault() to mark the agent row as a drop target; pre-fix this gate fails when dataTransfer.types lookup misses")
	}
	if !result.DropPrevented {
		t.Errorf("drop handler must call preventDefault()")
	}
	if len(result.Fetched) != 1 {
		t.Fatalf("expected exactly one POST after drop, got %d (fetched=%v)", len(result.Fetched), result.Fetched)
	}
	if !strings.Contains(result.Fetched[0].URL, "/api/items/BUG-999/assign") {
		t.Errorf("expected POST to /api/items/BUG-999/assign, got %q", result.Fetched[0].URL)
	}
	if !strings.Contains(result.Fetched[0].Body, "agent-target") {
		t.Errorf("expected request body to carry target agent id; got %q", result.Fetched[0].Body)
	}
}

// TestBoardDragDropTolerantOfEmptyTypesArray pins the cause-#1 fix:
// the drop-target gate must not depend on `dataTransfer.types`
// containing the custom MIME, because that lookup is browser-fragile
// during dragenter/dragover. Simulating an empty types array (the
// worst-case quirk), the dragover handler must still preventDefault
// because the same-tab `dragSourceItemId` flag is set.
func TestBoardDragDropTolerantOfEmptyTypesArray(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	const harness = `
function makeEl(tag) {
  return {
    _tag: tag, _handlers: {}, dataset: {},
    classList: { add:()=>{}, remove:()=>{}, toggle:()=>{}, contains:()=>false },
    children: [], style: {},
    appendChild(c) { this.children.push(c); return c; },
    removeAttribute() {}, setAttribute() {},
    addEventListener(t, fn) { this._handlers[t] = fn; },
    removeEventListener() {},
    querySelector: () => makeEl('div'), querySelectorAll: () => [],
    contains: () => false, insertAdjacentElement() {},
    getBoundingClientRect: () => ({ left:0, top:0, width:100, height:30 }),
  };
}
globalThis.document = {
  body: makeEl('body'), head: makeEl('head'),
  getElementById: () => makeEl('div'),
  querySelector: () => makeEl('div'), querySelectorAll: () => [],
  createElement: makeEl, createTextNode: (t) => ({ nodeValue: t }),
  addEventListener: () => {}, removeEventListener: () => {},
};
globalThis.window = globalThis;
globalThis.NodeFilter = { SHOW_TEXT: 4 };
globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({}) });

const board = await import('./board.js');
const li = makeEl('li');
li.dataset.state = 'idle';
board.wireIdleDrop(li, 'agent-x', 'X');
board.notifyDragStartForTest('BUG-100');

// Worst-case browser quirk: dataTransfer.types reports as empty
// during dragover even though setData was called. Pre-fix, the
// type-string check would fail and the row would silently be
// rejected as a drop target.
const emptyTypesDt = {
  types: [],
  getData: () => '',
  set effectAllowed(_) {}, set dropEffect(_) {},
};

const ev = { dataTransfer: emptyTypesDt, target: li, _defaultPrevented: false,
  preventDefault() { this._defaultPrevented = true; } };
li._handlers.dragover(ev);

process.stdout.write(JSON.stringify({ overPrevented: ev._defaultPrevented }));
`
	cmd := exec.Command("node", "--input-type=module", "--eval", harness)
	cmd.Dir = "web"
	stdout, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node harness failed: %v\nstderr: %s", err, stderr)
	}
	var result struct {
		OverPrevented bool `json:"overPrevented"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("decode: %v stdout=%s", err, stdout)
	}
	if !result.OverPrevented {
		t.Errorf("dragover must preventDefault even when dataTransfer.types is empty " +
			"— gate must use module-level drag state, not the type-array lookup")
	}
}

// TestBoardDragDropMarkPersistsAcrossChildHover pins the cause-#2 fix:
// the dragleave handler must use `relatedTarget` + `li.contains` to
// distinguish "cursor moved to a descendant" (still inside the row)
// from "cursor truly left the row". A bare `target !== li` guard
// leaves a frame-flicker as the cursor crosses descendant boundaries
// — the affordance disappears and (in some browsers) the row is no
// longer a valid drop target at the moment of release.
func TestBoardDragDropMarkPersistsAcrossChildHover(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	const harness = `
function makeEl(tag) {
  const el = {
    _tag: tag, _handlers: {}, dataset: {},
    classList: { add:()=>{}, remove:()=>{}, toggle:()=>{}, contains:()=>false },
    children: [], style: {},
    appendChild(c) { this.children.push(c); c._parent = this; return c; },
    removeAttribute(k) {
      const camel = k.replace(/^data-/, '').replace(/-(.)/g, (_, c) => c.toUpperCase());
      delete this.dataset[camel];
    },
    setAttribute() {},
    addEventListener(t, fn) { this._handlers[t] = fn; },
    removeEventListener() {},
    querySelector: () => makeEl('div'), querySelectorAll: () => [],
    contains(node) {
      let n = node;
      while (n) { if (n === this) return true; n = n._parent; }
      return false;
    },
    insertAdjacentElement() {},
    getBoundingClientRect: () => ({ left:0, top:0, width:100, height:30 }),
  };
  return el;
}
globalThis.document = {
  body: makeEl('body'), head: makeEl('head'),
  getElementById: () => makeEl('div'),
  querySelector: () => makeEl('div'), querySelectorAll: () => [],
  createElement: makeEl, createTextNode: (t) => ({ nodeValue: t }),
  addEventListener: () => {}, removeEventListener: () => {},
};
globalThis.window = globalThis;
globalThis.NodeFilter = { SHOW_TEXT: 4 };
globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({}) });

const board = await import('./board.js');
const li = makeEl('li');
const childBody = makeEl('div'); li.appendChild(childBody);
const outsideEl = makeEl('div');
board.wireIdleDrop(li, 'agent-x', 'X');
board.notifyDragStartForTest('BUG-100');

const dt = { types: ['application/x-squad-item'], getData: () => 'BUG-100',
  set effectAllowed(_) {}, set dropEffect(_) {} };

function fire(type, target, relatedTarget) {
  const ev = { type, target, relatedTarget, dataTransfer: dt, _defaultPrevented: false,
    preventDefault() { this._defaultPrevented = true; } };
  li._handlers[type](ev);
  return ev;
}

const stages = [];
// Cursor enters the row from outside.
fire('dragenter', li, outsideEl);
stages.push({ stage: 'after-enter-row', mark: li.dataset.dropTarget });
// Cursor moves from li-direct-area into a child: browser fires
// dragleave on li with relatedTarget=child, then bubbled dragenter
// on li from the child.
fire('dragleave', li, childBody);
stages.push({ stage: 'after-leave-to-child', mark: li.dataset.dropTarget });
fire('dragenter', childBody, li);
stages.push({ stage: 'after-enter-child', mark: li.dataset.dropTarget });
// Cursor truly leaves the row (from child to outside).
fire('dragleave', childBody, outsideEl);
fire('dragleave', li, outsideEl);
stages.push({ stage: 'after-leave-row', mark: li.dataset.dropTarget });

process.stdout.write(JSON.stringify({ stages }));
`
	cmd := exec.Command("node", "--input-type=module", "--eval", harness)
	cmd.Dir = "web"
	stdout, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node harness failed: %v\nstderr: %s", err, stderr)
	}
	var result struct {
		Stages []struct {
			Stage string `json:"stage"`
			Mark  string `json:"mark"`
		} `json:"stages"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("decode: %v stdout=%s", err, stdout)
	}

	wantSet := map[string]bool{
		"after-enter-row":      true,
		"after-leave-to-child": true,
		"after-enter-child":    true,
		"after-leave-row":      false,
	}
	for _, s := range result.Stages {
		got := s.Mark == "true"
		if got != wantSet[s.Stage] {
			t.Errorf("stage=%s: data-drop-target want=%v got mark=%q", s.Stage, wantSet[s.Stage], s.Mark)
		}
	}
}
