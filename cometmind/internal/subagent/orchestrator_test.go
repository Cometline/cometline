package subagent

import (
	"context"
	"testing"
	"time"
)

func TestOrchestrator_RegisterCompleteWait(t *testing.T) {
	o := NewOrchestrator(5)
	ctx := context.Background()
	_, cancel1 := context.WithCancel(ctx)
	_, cancel2 := context.WithCancel(ctx)
	defer cancel1()
	defer cancel2()

	if err := o.Register("parent", "child-1", KindGeneral, cancel1); err != nil {
		t.Fatal(err)
	}
	if err := o.Register("parent", "child-2", KindACP, cancel2); err != nil {
		t.Fatal(err)
	}

	done := make(chan []Result, 1)
	go func() {
		res, err := o.Wait(ctx, "parent", nil)
		if err != nil {
			t.Errorf("Wait() error = %v", err)
		}
		done <- res
	}()

	time.Sleep(20 * time.Millisecond)
	o.Complete("child-1", Result{Status: "completed", Summary: "one"})
	o.Complete("child-2", Result{Status: "completed", Summary: "two"})

	select {
	case res := <-done:
		if len(res) != 2 {
			t.Fatalf("got %d results want 2", len(res))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait timed out")
	}
}

func TestOrchestrator_MaxConcurrent(t *testing.T) {
	o := NewOrchestrator(2)
	_, c1 := context.WithCancel(context.Background())
	_, c2 := context.WithCancel(context.Background())
	_, c3 := context.WithCancel(context.Background())
	defer c1()
	defer c2()
	defer c3()

	if err := o.Register("p", "c1", KindGeneral, c1); err != nil {
		t.Fatal(err)
	}
	if err := o.Register("p", "c2", KindGeneral, c2); err != nil {
		t.Fatal(err)
	}
	if err := o.Register("p", "c3", KindGeneral, c3); err == nil {
		t.Fatal("expected max concurrent error")
	}
}

func TestOrchestrator_CancelChild(t *testing.T) {
	o := NewOrchestrator(5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Register("parent", "child", KindACP, cancel); err != nil {
		t.Fatal(err)
	}
	if !o.CancelChild("child") {
		t.Fatal("expected CancelChild to succeed")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected context cancelled")
	}
}

func TestOrchestrator_CancelForParent(t *testing.T) {
	o := NewOrchestrator(5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.Register("parent", "child", KindGeneral, cancel); err != nil {
		t.Fatal(err)
	}
	o.CancelForParent("parent")
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected context cancelled")
	}
}

func TestOrchestrator_WaitTimeout(t *testing.T) {
	o := NewOrchestrator(5)
	_, childCancel := context.WithCancel(context.Background())
	defer childCancel()

	if err := o.Register("parent", "child", KindGeneral, childCancel); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := o.Wait(ctx, "parent", []string{"child"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
