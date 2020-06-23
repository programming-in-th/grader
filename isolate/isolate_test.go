package isolate

import "testing"

func TestIsolate(t *testing.T) {
	instance := NewInstance("/usr/local/bin/isolate",
		0,
		"/home/proggrader/a.out",
		1,
		"/home/proggrader/logFile",
		1.0,
		5.0,
		512000,
		"/home/proggrader/output",
		"/home/proggrader/testcases/tasks/o61_may08_estate/inputs/19.in",
	)
	err := instance.Init()
	if err != nil {
		t.Log(err)
	}
	verdict, metrics := instance.Run()
	t.Log(verdict)
	t.Log(metrics)
	err = instance.Cleanup()
	if err != nil {
		t.Log(err)
	}
}
