package platform

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeOf(t *testing.T) {
	if ExitCodeOf(nil) != ExitOK {
		t.Fatal()
	}
	if ExitCodeOf(NewUsageError("x")) != ExitUsage {
		t.Fatal()
	}
	if ExitCodeOf(NewExitError(ExitSLOFail, fmt.Errorf("slo"))) != ExitSLOFail {
		t.Fatal()
	}
	if ExitCodeOf(NewExitError(ExitTempFail, errors.New("net"))) != ExitTempFail {
		t.Fatal()
	}
	if ExitCodeOf(errors.New("other")) != ExitSoftware {
		t.Fatal()
	}
}
