package subagent

import (
	"context"
	"sync"
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

func TestOrchestrator_WaitReturnsResultCompletedBeforeWait(t *testing.T) {
	o := NewOrchestrator(5)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := o.Register("parent", "child", KindGeneral, cancel); err != nil {
		t.Fatal(err)
	}
	o.Complete("child", Result{Status: "completed", Summary: "already done"})
	if got := o.ActiveCount("parent"); got != 0 {
		t.Fatalf("ActiveCount() = %d, want 0", got)
	}

	res, err := o.Wait(context.Background(), "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Summary != "already done" || res[0].ChildSessionID != "child" {
		t.Fatalf("Wait() = %+v, want the completed child", res)
	}
	again, err := o.Wait(context.Background(), "parent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second Wait() = %+v, want no replay", again)
	}
}

func TestOrchestrator_CompleteWaitRace(t *testing.T) {
	for i := 0; i < 50; i++ {
		o := NewOrchestrator(1)
		_, cancel := context.WithCancel(context.Background())
		if err := o.Register("parent", "child", KindGeneral, cancel); err != nil {
			cancel()
			t.Fatal(err)
		}
		var (
			wg  sync.WaitGroup
			res []Result
			err error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			res, err = o.Wait(context.Background(), "parent", nil)
		}()
		go func() {
			defer wg.Done()
			o.Complete("child", Result{Status: "completed", Summary: "raced"})
		}()
		wg.Wait()
		cancel()
		if err != nil || len(res) != 1 || res[0].Summary != "raced" {
			t.Fatalf("iter %d: Wait() = %+v, err = %v", i, res, err)
		}
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
